#!/bin/bash

# Initialize variables
destinationFile=""

# Parse arguments to find the -destination argument
for arg in "$@"; do
    if [[ "$arg" == "-destination" ]]; then
        # Handle the case where -destination is followed by a separate value
        nextIsDestination=true
    elif [[ "$arg" == -destination=* ]]; then
        # Handle the case where -destination=value is used
        destinationFile="${arg#-destination=}"
    elif [[ "$nextIsDestination" == true ]]; then
        # Capture the value after -destination
        destinationFile="$arg"
        nextIsDestination=false
    fi
done

# Ensure destinationFile is set
if [[ -z "$destinationFile" ]]; then
    echo "Error: -destination argument is required."
    exit 1
fi

# Check if destinationFile is newer than $GOFILE
if [[ "$destinationFile" -nt "$GOFILE" ]]; then
    exit 0
fi

# Execute the original mockgen command with all arguments
mockgen.local "$@" -write_command_comment=false