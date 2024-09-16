#!/bin/bash

set -e  # Exit immediately if a command exits with a non-zero status.

# Repository in the format "owner/repo"
REPO="ibeify/opsy-ami-operator"

# Number of hours to keep (delete runs older than this)
HOURS_TO_KEEP=2

# Number of concurrent deletions
CONCURRENCY=10

# Optional: Workflow ID (uncomment to use)
# WORKFLOW_ID="your_workflow_id"

# Function to calculate Unix timestamp
get_unix_timestamp() {
    if date -v -"${HOURS_TO_KEEP}H" >/dev/null 2>&1; then
        # macOS
        date -v -"${HOURS_TO_KEEP}H" +%s
    else
        # Linux and other Unix-like systems
        date -d "${HOURS_TO_KEEP} hours ago" +%s
    fi
}

# Calculate the cutoff date
CUTOFF_DATE=$(get_unix_timestamp)
echo "Cutoff date: $(date -d @$CUTOFF_DATE)"

# Function to delete a run
delete_run() {
    local run_number=$1
    local created_at=$2
    echo "Attempting to delete run $run_number from $created_at"
    gh run delete $run_number --repo $REPO
    if [ $? -eq 0 ]; then
        echo "Successfully deleted run $run_number from $created_at"
    else
        echo "Failed to delete run $run_number from $created_at"
    fi
}
export -f delete_run
export REPO

# Function to convert ISO 8601 date to Unix timestamp
iso8601_to_timestamp() {
    local iso_date=$1
    if date -j -f "%Y-%m-%dT%H:%M:%SZ" "$iso_date" +%s >/dev/null 2>&1; then
        # macOS
        date -j -f "%Y-%m-%dT%H:%M:%SZ" "$iso_date" +%s
    else
        # Linux and other Unix-like systems
        date -d "$iso_date" +%s
    fi
}
export -f iso8601_to_timestamp

# Fetch and process runs
fetch_and_process_runs() {
    local workflow_arg=""
    if [ ! -z "$WORKFLOW_ID" ]; then
        workflow_arg="--workflow $WORKFLOW_ID"
    fi

    echo "Fetching workflow runs..."
    local runs_json=$(gh run list --repo $REPO $workflow_arg --json databaseId,createdAt --limit 1000)
    echo "Fetched $(echo "$runs_json" | jq length) runs"

    echo "Filtering runs older than $HOURS_TO_KEEP hours..."
    local runs_to_delete=$(echo "$runs_json" | jq -r '.[] | [.databaseId, .createdAt] | @tsv' | while read -r id date; do
        timestamp=$(iso8601_to_timestamp "$date")
        if [ $timestamp -lt $CUTOFF_DATE ]; then
            echo "$id $date"
        fi
    done)

    local delete_count=$(echo "$runs_to_delete" | wc -l)
    echo "Found $delete_count runs to delete"

    if [ $delete_count -eq 0 ]; then
        echo "No runs to delete. Exiting."
        return
    fi

    echo "Starting parallel deletion..."
    echo "$runs_to_delete" | parallel --colsep ' ' -j $CONCURRENCY delete_run {1} {2}
}

# Check if required tools are installed
for cmd in gh jq parallel; do
    if ! command -v $cmd &> /dev/null; then
        echo "$cmd is not installed. Please install it and try again."
        exit 1
    fi
done

# Main execution
echo "Starting cleanup for $REPO"
echo "Deleting workflow runs older than $HOURS_TO_KEEP hours"
fetch_and_process_runs
echo "Cleanup completed"