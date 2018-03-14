#!/bin/bash
files=(jpegtest.jpg bmptest.bmp pngtest.png tifftest.tif )
tiffcompress=(lzw zip jpeg)

for f in "${files[@]}"; do
  convert -size 100x150 gradient: -rotate 90 \
    -sigmoidal-contrast 7x50% generated/"$f"
done

for c in "${tiffcompress[@]}"; do
  convert -compress "$c" generated/tifftest.tif generated/tifftest_"$c".tif
done
