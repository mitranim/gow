# Used for development: testing Linux without being on Linux.

from golang:alpine

workdir /gow

copy [".", "."]

run ["go", "mod", "download"]

# If everything works properly, then we should see a message about the FS event,
# and tests should rerun.
cmd sh -c "sleep 3 && echo 'package main' > touched.go & go run . -v test -v"
