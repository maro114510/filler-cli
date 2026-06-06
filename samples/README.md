# Sample Audio Files

This directory holds the three benchmark audio files used to validate filler detection accuracy.

## Recording Specifications

| Parameter | Requirement |
|-----------|-------------|
| Sample rate | 16 kHz |
| Channels | Mono (1 ch) |
| Environment | Quiet room, no background music |
| Duration | 60–180 seconds |

## Converting Existing Audio

If you have an `.m4a` or other format, convert it with [ffmpeg](https://ffmpeg.org/):

```bash
ffmpeg -i input.m4a -ar 16000 -ac 1 output.wav
```

## Sample Scenarios

### sample_a — Scripted, Low Filler

- **Expected filler count:** 0–3
- **Content:** A short scripted monologue read from notes (e.g., a product introduction or news summary)
- **Purpose:** Establishes the baseline detection rate for careful, prepared speech with minimal hesitation

Filename: `sample_a.wav`

---

### sample_b — Scripted, High Filler

- **Expected filler count:** 10–20
- **Content:** A scripted monologue that deliberately inserts filler words (「えー」「あのー」) approximately every 15–20 seconds
- **Purpose:** Verifies that `keepFillerToken=1` captures dense filler usage and that the breakdown table lists each word correctly

Filename: `sample_b.wav`

---

### sample_c — Spontaneous Technical Explanation

- **Duration:** 90–180 seconds
- **Expected filler count:** varies (natural rate for unscripted speech)
- **Content:** An impromptu explanation of a technical topic (e.g., how Git branching works, or how a microservice handles authentication) with no script or notes
- **Purpose:** Reflects real-world usage; the filler rate from this file is compared to the scripted samples to show detection across speech styles

Filename: `sample_c.wav`

---

## Adding Samples

1. Record or convert your audio to 16 kHz mono WAV (see above).
2. Place the file in this directory as `sample_a.wav`, `sample_b.wav`, or `sample_c.wav`.
3. Verify with `filler-cli analyze`:

   ```bash
   filler-cli analyze samples/sample_a.wav
   filler-cli analyze samples/sample_b.wav
   filler-cli analyze samples/sample_c.wav
   ```

4. Record ground-truth timestamps in `annotations/` using the schema at [`annotations/schema.json`](../annotations/schema.json).

> **Note:** Audio files are excluded from the repository via `.gitignore`. Commit only the annotation JSON files.
