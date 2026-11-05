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
printf '%s\n' 'Exact source packages must accompany public image distribution. See the source archive for this image digest; a package homepage alone is not a source offer.' > "$out/README.txt"
