package handlers

import (
	"net/http"
	"strings"
	"time"

	"pulsepoll/backend/internal/config"
	"pulsepoll/backend/internal/models"
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
	polls *mongo.Collection
	votes *mongo.Collection
	hub   *realtime.Hub
	cfg   *config.Config
}

func NewPollHandler(db *mongo.Database, hub *realtime.Hub, cfg *config.Config) *PollHandler {
	return &PollHandler{
		polls: db.Collection("polls"),
		votes: db.Collection("votes"),
		hub:   hub,
		cfg:   cfg,
	}
}

type createReq struct {
	Title       string   `json:"title"       binding:"required,min=3,max=140"`
	Description string   `json:"description" binding:"max=500"`
	Options     []string `json:"options"     binding:"required,min=2,max=10,dive,required,min=1,max=80"`
}

func (h *PollHandler) CreatePoll(c *gin.Context) {
	var req createReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	req.Description = strings.TrimSpace(req.Description)

	options := make([]models.Option, 0, len(req.Options))
	for _, raw := range req.Options {
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
			Title:       req.Title,
			Description: req.Description,
			Options:     options,
			CreatedBy:   userID.(primitive.ObjectID),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		_, err = h.polls.InsertOne(c.Request.Context(), poll)
		if err != nil {
			if mongo.IsDuplicateKeyError(err) {
				continue
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, pollViewFromPoll(&poll, 0, nil))
		return
	}
	c.JSON(http.StatusConflict, gin.H{"error": "could not generate unique slug, try again"})
}

func (h *PollHandler) GetMine(c *gin.Context) {
	userID, _ := c.Get("user_id")
	ctx := c.Request.Context()

	cur, err := h.polls.Find(ctx,
		bson.M{"created_by": userID},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}),
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cur.Close(ctx)

	var polls []models.Poll
	if err := cur.All(ctx, &polls); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	type pollWithCounts struct {
		models.Poll `bson:",inline"`
		TotalVotes  int64            `json:"totalVotes"`
		LiveCounts  map[string]int64 `json:"liveCounts"`
	}
	results := make([]pollWithCounts, 0, len(polls))
	for _, p := range polls {
		counts, total, _ := h.hub.SnapshotCounts(ctx, p.Slug)
		results = append(results, pollWithCounts{Poll: p, TotalVotes: total, LiveCounts: counts})
	}
	c.JSON(http.StatusOK, results)
}

func (h *PollHandler) GetPoll(c *gin.Context) {
	slug := c.Param("slug")
	ctx := c.Request.Context()

	var poll models.Poll
	if err := h.polls.FindOne(ctx, bson.M{"slug": slug}).Decode(&poll); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "poll not found"})
		return
	}
	counts, total, _ := h.hub.SnapshotCounts(ctx, slug)
	c.JSON(http.StatusOK, pollViewFromPoll(&poll, total, counts))
}

func (h *PollHandler) Vote(c *gin.Context) {
	slug := c.Param("slug")
	var req struct {
		OptionID string `json:"optionId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.OptionID = strings.TrimSpace(req.OptionID)
	ctx := c.Request.Context()

	var poll models.Poll
	if err := h.polls.FindOne(ctx, bson.M{"slug": slug}).Decode(&poll); err != nil {
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

	// Voter dedup via anonymous cookie
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

	snap, isNew, err := h.hub.IncrVote(ctx, slug, req.OptionID, voterID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "vote failed: " + err.Error()})
		return
	}
	if !isNew {
		c.JSON(http.StatusConflict, gin.H{"error": "you have already voted on this poll"})
		return
	}

	// Durable vote record in Mongo
	h.votes.InsertOne(ctx, models.Vote{
		ID:        primitive.NewObjectID(),
		PollID:    slug,
		OptionID:  req.OptionID,
		VoterID:   voterID,
		UserAgent: c.Request.UserAgent(),
		CreatedAt: time.Now(),
	})

	// Durable count in Mongo via arrayFilters
	h.polls.UpdateByID(ctx, poll.ID, bson.M{
		"$inc": bson.M{"options.$[elem].count": 1},
	}, options.Update().SetArrayFilters(options.ArrayFilters{
		Filters: bson.A{bson.M{"elem.id": req.OptionID}},
	}))

	c.JSON(http.StatusOK, gin.H{
		"counts": snap.Counts,
		"total":  snap.Total,
	})
}

func (h *PollHandler) ClosePoll(c *gin.Context) {
	slug := c.Param("slug")
	userID, _ := c.Get("user_id")
	ctx := c.Request.Context()

	var poll models.Poll
	if err := h.polls.FindOne(ctx, bson.M{"slug": slug}).Decode(&poll); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "poll not found"})
		return
	}
	if poll.CreatedBy != userID.(primitive.ObjectID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "not your poll"})
		return
	}

	newClosed := !poll.Closed
	if _, err := h.polls.UpdateByID(ctx, poll.ID, bson.M{
		"$set": bson.M{"closed": newClosed, "updated_at": time.Now()},
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if newClosed {
		_ = h.hub.PublishClosed(ctx, slug)
	}
	c.JSON(http.StatusOK, gin.H{"closed": newClosed})
}

func (h *PollHandler) DeletePoll(c *gin.Context) {
	slug := c.Param("slug")
	userID, _ := c.Get("user_id")
	ctx := c.Request.Context()

	res, err := h.polls.DeleteOne(ctx, bson.M{"slug": slug, "created_by": userID})
	if err != nil || res.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "poll not found or not yours"})
		return
	}
	h.votes.DeleteMany(ctx, bson.M{"poll_id": slug})
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func pollViewFromPoll(p *models.Poll, total int64, counts map[string]int64) models.PollView {
	return models.PollView{
		Poll:       *p,
		TotalVotes: total,
		LiveCounts: counts,
	}
}
