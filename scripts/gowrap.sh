#!/bin/bash

# Initialize variables
destinationFile=""
templateFile=""

# Parse arguments to find the -o (output) and -t (template) arguments
for ((i = 1; i <= $#; i++)); do
    if [[ "${!i}" == "-o" ]]; then
        # Capture the value of the -o argument (destination file)
        nextIndex=$((i + 1))
        destinationFile="${!nextIndex}"
    elif [[ "${!i}" == "-t" ]]; then
        # Capture the value of the -t argument (template file)
        nextIndex=$((i + 1))
        templateFile="${!nextIndex}"
    fi
done

# Ensure destinationFile and templateFile are set
if [[ -z "$destinationFile" ]]; then
    echo "Error: -o (output) argument is required."
    exit 1
fi

if [[ -z "$templateFile" ]]; then
    echo "Error: -t (template) argument is required."
    exit 1
fi

# Check if destinationFile is newer than $GOFILE and templateFile
if [[ "$destinationFile" -nt "$GOFILE" && "$destinationFile" -nt "$templateFile" ]]; then
    echo "Skipped! $GOFILE"
    exit 0
fi

echo "Regenerating. $GOFILE"
# Execute the original gowrap command with all arguments
gowrap.local "$@"