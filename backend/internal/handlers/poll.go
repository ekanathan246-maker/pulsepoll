package handlers

import (
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"pulsepoll/backend/internal/config"
	"pulsepoll/backend/internal/domain"
	"pulsepoll/backend/internal/middleware"
	"pulsepoll/backend/internal/models"
	"pulsepoll/backend/internal/platform/httpx"
	"pulsepoll/backend/internal/realtime"
	"pulsepoll/backend/internal/utils"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// PollHandler serves poll CRUD and voting.
type PollHandler struct {
	db     *mongo.Database
	polls  *mongo.Collection
	votes  *mongo.Collection
	outbox *mongo.Collection
	hub    *realtime.Hub
	cfg    *config.Config
}

func NewPollHandler(db *mongo.Database, hub *realtime.Hub, cfg *config.Config) *PollHandler {
	return &PollHandler{
		db:     db,
		polls:  db.Collection("polls"),
		votes:  db.Collection("votes"),
		outbox: db.Collection("outbox"),
		hub:    hub,
		cfg:    cfg,
	}
}

type createReq struct {
	Title             string     `json:"title"`
	Description       string     `json:"description"`
	Options           []string   `json:"options"`
	ClosesAt          *time.Time `json:"closesAt"`
	ShowResultsBefore bool       `json:"showResultsBeforeVote"`
}

func (h *PollHandler) CreatePoll(c *gin.Context) {
	var req createReq
	if err := httpx.DecodeJSON(c, &req, 32<<10); err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorBody("invalid_json", "Send a valid JSON poll."))
		return
	}
	validated, validationErr := (domain.NewPollInput{
		Question: req.Title, Description: req.Description, Options: req.Options,
		ClosesAt: req.ClosesAt, ShowResultsBefore: req.ShowResultsBefore,
	}).Validate(time.Now())
	if validationErr != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorBody(validationErr.Code, validationErr.Message))
		return
	}

	options := make([]models.Option, 0, len(req.Options))
	for _, raw := range validated.Options {
		optID, err := utils.NewOptionID()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate option id"})
			return
		}
		options = append(options, models.Option{ID: optID, Text: strings.TrimSpace(raw)})
	}

	userID, _ := c.Get("user_id")

	var slug string
	for attempt := 0; attempt < 5; attempt++ {
		s, err := utils.NewSlug()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate slug"})
			return
		}
		slug = s
		poll := models.Poll{
			ID:          primitive.NewObjectID(),
			Slug:        slug,
			Title:       validated.Question,
			Description: validated.Description,
			Options:     options,
			CreatedBy:   userID.(primitive.ObjectID),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
			Version:     1,
			ClosesAt:    validated.ClosesAt,
			HideResults: !validated.ShowResultsBefore,
		}
		_, err = h.polls.InsertOne(c.Request.Context(), poll)
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				continue
			}
			c.JSON(http.StatusInternalServerError, middleware.ErrorBody("poll_not_created", "Poll could not be created. Try again."))
			return
		}
		c.JSON(http.StatusCreated, pollViewFromPoll(&poll, true))
		return
	}
	c.JSON(http.StatusConflict, gin.H{"error": "could not generate unique slug, try again"})
}

func (h *PollHandler) GetMine(c *gin.Context) {
	userID, _ := c.Get("user_id")
	ctx := c.Request.Context()

	cur, err := h.polls.Find(ctx,
		bson.M{"created_by": userID, "deleted_at": bson.M{"$exists": false}},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, middleware.ErrorBody("polls_unavailable", "Polls are temporarily unavailable."))
		return
	}
	defer cur.Close(ctx)

	var polls []models.Poll
	if err := cur.All(ctx, &polls); err != nil {
		c.JSON(http.StatusInternalServerError, middleware.ErrorBody("polls_unavailable", "Polls are temporarily unavailable."))
		return
	}

	results := make([]models.PollView, 0, len(polls))
	for i := range polls {
		results = append(results, pollViewFromPoll(&polls[i], true))
	}
	c.JSON(http.StatusOK, results)
}

func (h *PollHandler) GetPoll(c *gin.Context) {
	slug := c.Param("slug")
	ctx := c.Request.Context()

	var poll models.Poll
	if err := h.polls.FindOne(ctx, bson.M{"slug": slug, "deleted_at": bson.M{"$exists": false}}).Decode(&poll); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "poll not found"})
		return
	}
	ownerID, ownerAuthenticated := c.Get("user_id")
	canViewResults := ownerAuthenticated && ownerID == poll.CreatedBy
	canViewResults = canViewResults || poll.Closed || !poll.HideResults || (poll.ClosesAt != nil && !poll.ClosesAt.After(time.Now().UTC()))
	if !canViewResults {
		if voterID, err := c.Cookie("ppv"); err == nil && voterID != "" {
			count, countErr := h.votes.CountDocuments(ctx, bson.M{"poll_id": poll.ID, "voter_id": voterID}, options.Count().SetLimit(1))
			canViewResults = countErr == nil && count > 0
		}
	}
	if canViewResults {
		if hotCounts, hotVersion, _, hotErr := h.hub.DurableSnapshot(ctx, slug); hotErr == nil && hotVersion == poll.Version {
			for i := range poll.Options {
				poll.Options[i].Count = hotCounts[poll.Options[i].ID]
			}
		} else {
			// Never jump Redis ahead of pending outbox events: doing so would make
			// their idempotency guard suppress legitimate WebSocket updates.
			pending, pendingErr := h.outbox.CountDocuments(ctx, bson.M{
				"aggregate_id": slug, "processed_at": bson.M{"$exists": false},
			}, options.Count().SetLimit(1))
			if pendingErr == nil && pending == 0 {
				counts, _ := countsFromPoll(&poll)
				// Cache repair is best-effort: Redis must never make the durable read fail.
				_ = h.hub.RepairSnapshot(ctx, slug, poll.Version, counts, pollStatus(&poll))
			}
		}
	}
	c.JSON(http.StatusOK, pollViewFromPoll(&poll, canViewResults))
}

func (h *PollHandler) Vote(c *gin.Context) {
	slug := c.Param("slug")
	var req struct {
		OptionID string `json:"optionId" binding:"required"`
	}
	if err := httpx.DecodeJSON(c, &req, 8<<10); err != nil {
		c.JSON(http.StatusBadRequest, middleware.ErrorBody("invalid_request", "Choose a valid poll option."))
		return
	}
	req.OptionID = strings.TrimSpace(req.OptionID)
	ctx := c.Request.Context()

	var poll models.Poll
	if err := h.polls.FindOne(ctx, bson.M{"slug": slug, "deleted_at": bson.M{"$exists": false}}).Decode(&poll); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "poll not found"})
		return
	}
	if poll.Closed {
		c.JSON(http.StatusConflict, gin.H{"error": "poll is closed"})
		return
	}

	valid := false
	for _, o := range poll.Options {
		if o.ID == req.OptionID {
			valid = true
			break
		}
	}
	if !valid {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid option id"})
		return
	}

	// Voter identity is abuse friction, not proof of one human. The durable
	// unique index on (poll_id,voter_id) is the final duplicate-vote guard.
	voterID, err := c.Cookie("ppv")
	if err != nil || voterID == "" {
		voterID, err = utils.NewVoterID()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate voter id"})
			return
		}
		secure := h.cfg.IsProduction()
		sameSite := http.SameSiteLaxMode
		if secure {
			sameSite = http.SameSiteNoneMode
		}
		c.SetSameSite(sameSite)
		c.SetCookie("ppv", voterID, 90*24*3600, "/", "", secure, true)
	}

	now := time.Now().UTC()
	eventID, err := utils.NewEventID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, middleware.ErrorBody("internal_error", "Vote could not be accepted."))
		return
	}
	var updated models.Poll
	session, err := h.db.Client().StartSession()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, middleware.ErrorBody("database_unavailable", "Voting is temporarily unavailable. Try again."))
		return
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, func(tx mongo.SessionContext) (any, error) {
		_, err := h.votes.InsertOne(tx, models.Vote{
			ID: primitive.NewObjectID(), PollID: poll.ID, OptionID: req.OptionID,
			VoterID: voterID, UserAgent: c.Request.UserAgent(), CreatedAt: now,
		})
		if err != nil {
			return nil, err
		}
		filter := bson.M{
			"_id": poll.ID, "closed": false, "deleted_at": bson.M{"$exists": false}, "options.id": req.OptionID,
			"$or": bson.A{bson.M{"closes_at": bson.M{"$exists": false}}, bson.M{"closes_at": bson.M{"$gt": now}}},
		}
		update := bson.M{
			"$inc": bson.M{"options.$[elem].count": 1, "version": 1},
			"$set": bson.M{"updated_at": now},
		}
		err = h.polls.FindOneAndUpdate(tx, filter, update, options.FindOneAndUpdate().
			SetArrayFilters(options.ArrayFilters{Filters: bson.A{bson.M{"elem.id": req.OptionID}}}).
			SetReturnDocument(options.After)).Decode(&updated)
		if err != nil {
			return nil, err
		}
		counts, _ := countsFromPoll(&updated)
		_, err = h.outbox.InsertOne(tx, models.OutboxEvent{
			ID: primitive.NewObjectID(), EventID: eventID, AggregateID: slug,
			Type: "vote.applied", Version: updated.Version, OptionID: req.OptionID,
			Delta: 1, Counts: counts, Status: pollStatus(&updated), CreatedAt: now, NextAttemptAt: now,
		})
		return nil, err
	})
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			c.JSON(http.StatusConflict, middleware.ErrorBody("duplicate_vote", "This browser has already voted in this poll."))
			return
		}
		if errors.Is(err, mongo.ErrNoDocuments) {
			c.JSON(http.StatusConflict, middleware.ErrorBody("poll_closed", "This poll is closed."))
			return
		}
		c.JSON(http.StatusServiceUnavailable, middleware.ErrorBody("vote_not_accepted", "Your vote was not accepted. Try again."))
		return
	}
	counts, total := countsFromPoll(&updated)
	c.JSON(http.StatusAccepted, gin.H{"accepted": true, "version": updated.Version, "counts": counts, "total": total})
}

func (h *PollHandler) ClosePoll(c *gin.Context) {
	slug := c.Param("slug")
	userID, _ := c.Get("user_id")
	ctx := c.Request.Context()

	var poll models.Poll
	if err := h.polls.FindOne(ctx, bson.M{"slug": slug, "deleted_at": bson.M{"$exists": false}}).Decode(&poll); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "poll not found"})
		return
	}
	if poll.CreatedBy != userID.(primitive.ObjectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not your poll"})
		return
	}

	newClosed := !poll.Closed
	now := time.Now().UTC()
	eventID, err := utils.NewEventID()
	if err != nil {
		c.JSON(http.StatusInternalServerError, middleware.ErrorBody("internal_error", "Poll status could not be changed."))
		return
	}
	var updated models.Poll
	session, err := h.db.Client().StartSession()
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, middleware.ErrorBody("database_unavailable", "Poll status could not be changed."))
		return
	}
	defer session.EndSession(ctx)
	_, err = session.WithTransaction(ctx, func(tx mongo.SessionContext) (any, error) {
		update := bson.M{
			"$set": bson.M{"closed": newClosed, "updated_at": now},
			"$inc": bson.M{"version": 1},
		}
		// Reopening a poll with an expired schedule must also clear that schedule;
		// otherwise its computed status would remain closed even after toggling.
		if !newClosed && poll.ClosesAt != nil && !poll.ClosesAt.After(now) {
			update["$unset"] = bson.M{"closes_at": ""}
		}
		err := h.polls.FindOneAndUpdate(tx,
			bson.M{"_id": poll.ID, "created_by": userID, "deleted_at": bson.M{"$exists": false}},
			update,
			options.FindOneAndUpdate().SetReturnDocument(options.After),
		).Decode(&updated)
		if err != nil {
			return nil, err
		}
		counts, _ := countsFromPoll(&updated)
		eventType := "poll.reopened"
		if newClosed {
			eventType = "poll.closed"
		}
		_, err = h.outbox.InsertOne(tx, models.OutboxEvent{
			ID: primitive.NewObjectID(), EventID: eventID, AggregateID: slug,
			Type: eventType, Version: updated.Version, Counts: counts,
			Status: pollStatus(&updated), CreatedAt: now, NextAttemptAt: now,
		})
		return nil, err
	})
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, middleware.ErrorBody("status_not_changed", "Poll status could not be changed. Try again."))
		return
	}
	c.JSON(http.StatusOK, gin.H{"closed": pollStatus(&updated) == "closed", "version": updated.Version})
}

func (h *PollHandler) DeletePoll(c *gin.Context) {
	slug := c.Param("slug")
	userID, _ := c.Get("user_id")
	ctx := c.Request.Context()

	now := time.Now().UTC()
	res, err := h.polls.UpdateOne(ctx,
		bson.M{"slug": slug, "created_by": userID, "deleted_at": bson.M{"$exists": false}},
		bson.M{"$set": bson.M{"deleted_at": now, "closed": true, "updated_at": now}, "$inc": bson.M{"version": 1}},
	)
	if err != nil || res.ModifiedCount == 0 {
		c.JSON(http.StatusNotFound, middleware.ErrorBody("poll_not_found", "Poll not found."))
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "archived"})
}

func (h *PollHandler) ExportCSV(c *gin.Context) {
	slug := c.Param("slug")
	userID, _ := c.Get("user_id")
	var poll models.Poll
	if err := h.polls.FindOne(c.Request.Context(), bson.M{
		"slug": slug, "created_by": userID, "deleted_at": bson.M{"$exists": false},
	}).Decode(&poll); err != nil {
		c.JSON(http.StatusNotFound, middleware.ErrorBody("poll_not_found", "Poll not found."))
		return
	}
	_, total := countsFromPoll(&poll)
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", `attachment; filename="pulsepoll-`+slug+`.csv"`)
	writer := csv.NewWriter(c.Writer)
	_ = writer.Write([]string{"option_id", "option", "votes", "percentage"})
	for _, option := range poll.Options {
		percentage := float64(0)
		if total > 0 {
			percentage = float64(option.Count) / float64(total) * 100
		}
		_ = writer.Write([]string{option.ID, option.Text, strconv.FormatInt(option.Count, 10), fmt.Sprintf("%.1f", percentage)})
	}
	writer.Flush()
}

func pollViewFromPoll(p *models.Poll, canViewResults bool) models.PollView {
	counts, total := countsFromPoll(p)
	copyPoll := *p
	copyPoll.Options = append([]models.Option(nil), p.Options...)
	if copyPoll.ClosesAt != nil && !copyPoll.ClosesAt.After(time.Now().UTC()) {
		copyPoll.Closed = true
		canViewResults = true
	}
	if !canViewResults {
		counts = nil
		total = 0
		for i := range copyPoll.Options {
			copyPoll.Options[i].Count = 0
		}
	}
	return models.PollView{
		Poll:           copyPoll,
		TotalVotes:     total,
		LiveCounts:     counts,
		CanViewResults: canViewResults,
	}
}

func countsFromPoll(p *models.Poll) (map[string]int64, int64) {
	counts := make(map[string]int64, len(p.Options))
	var total int64
	for _, option := range p.Options {
		counts[option.ID] = option.Count
		total += option.Count
	}
	return counts, total
}

func pollStatus(p *models.Poll) string {
	if p.Closed || (p.ClosesAt != nil && !p.ClosesAt.After(time.Now().UTC())) {
		return "closed"
	}
	return "open"
}
