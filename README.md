# filler-cli

Analyze filler words in Japanese speech using the AmiVoice speech recognition API.

## Installation

### go install

Requires Go 1.26 or later.

```bash
go install github.com/maro114510/filler-cli@latest
```

### Binary download

Pre-built binaries for Linux, macOS (Intel/Apple Silicon), and Windows are available on the [Releases page](https://github.com/maro114510/filler-cli/releases).

Download and extract the archive for your platform, then place `filler-cli` somewhere in your `$PATH`.

## Prerequisites

An AmiVoice API key from the [AmiVoice Cloud Console](https://acp.amivoice.com/main/register/).

> **Note:** Use a standard (non-End-to-End) engine. End-to-End engines suppress filler tokens and will not work with this tool.

## API Key Setup

The CLI checks for your API key in this order:

1. **Environment variable** — set `AMIVOICE_API_KEY` (recommended for CI and scripts):

   ```bash
   export AMIVOICE_API_KEY=your_key_here
   filler-cli analyze speech.wav
   ```

   Copy `.env.example` to `.env`, fill in your key, and load it with `source .env` or [direnv](https://direnv.net/).

2. **Interactive prompt + keystore** — if the env var is not set, the CLI prompts once and caches the key in `~/.config/filler-cli/credentials.json` for 2 hours.

## Quickstart

```bash
filler-cli analyze speech.wav
```

Expected output:

```
# Filler Analysis: speech.wav

## Estimated Speech Duration

92.3 s

## Total Fillers

7

## Fillers per Minute

4.55

## Filler Breakdown

| Filler | Count |
|--------|-------|
| えーと | 4 |
| あのー | 3 |

## Filler Event Timeline

| Filler | Start (ms) | End (ms) | Confidence |
|--------|-----------|---------|------------|
| えーと | 3120 | 3890 | 0.95 |
| あのー | 8440 | 9200 | 0.91 |
...
```

## Commands

### analyze

```
filler-cli analyze [flags] <audio-file>
```

Transcribes the audio file with AmiVoice and reports filler words found.

**Supported formats:** `.wav`, `.mp3`

**Flags:**

| Flag | Default | Description |
|------|---------|-------------|
| `--format` | `markdown` | Output format: `json` or `markdown` |
| `--output` | *(stdout)* | Write output to a file instead of stdout |
| `--keep-filler-token` | `1` | Pass `keepFillerToken` to AmiVoice (`0` or `1`). Set to `1` to detect fillers. |

**JSON output example:**

```bash
filler-cli analyze --format json speech.wav
```

```json
{
  "audioFile": "speech.wav",
  "durationSec": 92.3,
  "generatedAt": "2026-06-06T10:00:00Z",
  "totalFillers": 7,
  "fillersPerMinute": 4.55,
  "breakdown": { "えーと": 4, "あのー": 3 },
  "firstFillerTimeMs": 3120,
  "fillerEvents": [
    { "displayName": "えーと", "startMs": 3120, "endMs": 3890, "confidence": 0.95 }
  ],
  "averageConfidence": 0.93
}
```

**Save to file:**

```bash
filler-cli analyze --format json --output report.json speech.wav
```

### version

```
filler-cli version
```

Print the installed version.

### Global flags

| Flag | Default | Description |
|------|---------|-------------|
| `--debug` | `false` | Enable debug output |

## Raw curl Example

The following reproduces what `filler-cli analyze` sends to AmiVoice, without the CLI:

```bash
curl -s -X POST https://acp-api.amivoice.com/v1/recognize \
  -F "u=YOUR_API_KEY" \
  -F "d=grammarFileNames=-a-general keepFillerToken=1" \
  -F "a=@speech.wav;type=audio/wav"
```

| Form field | Value | Description |
|------------|-------|-------------|
| `u` | API key | Authentication |
| `d` | `grammarFileNames=-a-general keepFillerToken=1` | Engine and filler-token options |
| `a` | Audio file (`@filename`) | Must be the last field |

## Sample Audio

See [`samples/README.md`](samples/README.md) for recording specifications and the three benchmark scenarios used to validate filler detection.

## Manual Annotations

The `annotations/` directory holds ground-truth filler timestamps in JSON format. See [`annotations/schema.json`](annotations/schema.json) for the schema definition.

## Reproducing Results

After placing audio files in `samples/` (see [`samples/README.md`](samples/README.md)), run:

```bash
make results
```

This produces six JSON files under `results/`:

| File | keepFillerToken |
|------|----------------|
| `results/sample-a-filler0.result.json` | 0 |
| `results/sample-a-filler1.result.json` | 1 |
| `results/sample-b-filler0.result.json` | 0 |
| `results/sample-b-filler1.result.json` | 1 |
| `results/sample-c-filler0.result.json` | 0 |
| `results/sample-c-filler1.result.json` | 1 |

## Limitations

The following limitations were identified during live verification (see [Epic Issue #1](https://github.com/maro114510/filler-cli/issues/1) Kill Criteria).

### KC-001 — Filler token availability

`keepFillerToken=1` requires the `-a-general` (Hybrid) engine.
End-to-End engines suppress `%...%` tokens and will produce zero filler events.
If `fillerEvents` is empty on `sample_b` despite audible fillers, verify that the engine is set to `-a-general`.

### KC-002 — Timestamp stability

Token-level `startTime`/`endTime` fields are provided on a best-effort basis by the AmiVoice API.
In rare cases these values may be zero or absent for short filler tokens.
When this occurs, `firstFillerTimeMs` and the timeline table will be incomplete.

### KC-003 — Manual vs. API count discrepancy

The API counts only tokens whose `written` field matches `%...%` (e.g., `%えー%`, `%あのー%`).
Demonstratives (`その`), affirmatives (`はい`), and contextual fillers (`まあ`) are **not** counted.
A gap between the manual annotation count and the API count is expected for spontaneous speech and reflects this definition boundary, not a detection error.

## License

Apache-2.0 — see [LICENSE](LICENSE) for details.
