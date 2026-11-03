# Video player sample

`player.mp4` is a synthetic 20-second test pattern and sine wave generated for
this project, with no third-party media content. Reproduce it using FFmpeg:

```sh
ffmpeg -f lavfi -i 'testsrc2=size=320x180:rate=12' \
  -f lavfi -i 'sine=frequency=440:sample_rate=44100' -t 20 \
  -c:v libx264 -preset ultrafast -crf 35 -pix_fmt yuv420p \
  -c:a aac -b:a 32k -movflags frag_keyframe+empty_moov -f mp4 player.mp4
```

The browser test serves byte ranges from this fixture to exercise actual decoding,
playback and seeking, without depending on external media URLs.

`player-portrait.mp4` uses the same testsrc2 generator with `size=180x320`,
a 3-second duration and no audio input, to verify portrait layout and controls.
