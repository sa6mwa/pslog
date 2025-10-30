#!/bin/sh

#vhs pitch.vhs

set -e

ffmpeg -i elevatorpitch.gif -filter_complex \
"[0:v]mpdecimate,fps=12,split[a][b];[a]palettegen=stats_mode=diff[p];[b][p]paletteuse=dither=bayer:bayer_scale=5" \
-loop 0 output.gif
gifsicle -O3 --lossy=80 -o out.gif output.gif
rm -f output.gif
