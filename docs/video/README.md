# Demo video artifact

`NARRATION.md` is a truthful 3–5 minute walkthrough aligned with the submission rubric. It deliberately describes the measured local environment and does not claim a public deployment.

On macOS, generate the fallback MP4 from the checked-in screenshots and evidence:

```bash
python3 scripts/build_demo_video.py
```

The output is `dist/pulsepoll-engineering-demo.mp4`. The script uses the built-in `say` voice plus ffmpeg, so it requires no API key or paid service. The generated voice is a fallback: the candidate should replace it with their own narration when possible so they can demonstrate understanding naturally in the interview.

Before submitting, watch the complete export, upload it somewhere the reviewer can access, and re-check its permissions. Do not call the video a live-deployment demo until a public URL has been provisioned and verified.
