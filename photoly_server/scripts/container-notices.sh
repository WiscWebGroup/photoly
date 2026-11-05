#!/bin/sh
set -eu
out=/usr/share/photoly/licenses
mkdir -p "$out/system" /usr/share/photoly/sources
dpkg-query -W -f='${db:Status-Status}\t${binary:Package}\t${Version}\t${source:Package}\t${source:Version}\n' | awk -F '\t' '$1 == "installed" {sub(/^[^\t]*\t/, ""); print}' > "$out/system-packages.tsv"
while IFS="$(printf '\t')" read -r package version source source_version; do
 name=${package%%:*}
 if [ -f "/usr/share/doc/$name/copyright" ]; then
  cat "/usr/share/doc/$name/copyright" > "$out/system/$name.copyright"
 else
  printf '%s\t%s\n' "$package" 'No standalone copyright file; inspect distribution package source' >> "$out/system-notice-review.tsv"
 fi
done < "$out/system-packages.tsv"
cp -a /usr/share/common-licenses "$out/"
printf '%s\n' 'Source for GPL/LGPL and other Debian packages in this image: the exact package versions are listed in system-packages.tsv. Corresponding source is available from the Debian archive (https://snapshot.debian.org/) and, on request, from the Photoly maintainers at https://github.com/WiscWebGroup/photoly/issues for at least three years after the image is distributed. Photoly, Go module and pgvector sources are in /usr/share/photoly/sources and at the source address shown on the /legal page.' > "$out/README.txt"
