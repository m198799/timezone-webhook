#!/bin/bash

# remove all blank lines in go 'imports' statements
for file in $(find . -name '*.go'); do
  #  echo "goimports helper doing $file"
  if [[ $(uname) == 'Darwin' ]]; then
    sed -i '' '
          /^import/,/)/ {
            /^$/ d
          }
        ' "$file"
  elif [[ $(uname) == 'Linux' ]]; then
    sed -i '
          /^import/,/)/ {
            /^$/ d
          }
        ' "$file"
  else
    echo "$(uname) arch is not supported yet"
  fi
done
