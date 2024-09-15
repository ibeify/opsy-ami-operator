#!/bin/bash

# Ensure gh CLI is installed and authenticated
if ! command -v gh &> /dev/null; then
    echo "GitHub CLI (gh) is not installed. Please install it first."
    exit 1
fi

# Check if a repository is provided
if [ $# -eq 0 ]; then
    echo "Usage: $0 <owner/repo>"
    exit 1
fi

REPO=$1

# Function to get creation date of a tag
get_tag_date() {
    local tag=$1
    gh api -X GET "repos/$REPO/git/refs/tags/$tag" --jq '.object.url' | xargs gh api --jq '.author.date'
}

# Get all releases with their creation dates
releases=$(gh release list -R "$REPO" --limit 1000 --json tagName,createdAt \
           | jq -r '.[] | [.createdAt, .tagName] | @tsv' \
           | sort -r)

# Get all tags
all_tags=$(gh api -X GET "repos/$REPO/git/refs/tags" --jq '.[].ref' | cut -d'/' -f3-)

# Combine releases and tags, sort by date
combined_list=$(
    (echo "$releases";
     comm -13 <(echo "$releases" | cut -f2) <(echo "$all_tags" | sort) | while read -r tag; do
         date=$(get_tag_date "$tag")
         echo -e "$date\t$tag"
     done) | sort -r
)

# Count the total number of items
total_count=$(echo "$combined_list" | wc -l)

# If there are 2 or fewer items, exit
if [ "$total_count" -le 2 ]; then
    echo "There are 2 or fewer releases/tags. No deletions needed."
    exit 0
fi

# Calculate how many items to delete
to_delete=$((total_count - 2))

# Delete all but the last 2 items (releases or tags)
echo "$combined_list" | tail -n "$to_delete" | while read -r date item; do
    if gh release view "$item" -R "$REPO" &>/dev/null; then
        echo "Deleting release and tag: $item (created on $date)"
        gh release delete "$item" -R "$REPO" --yes
    else
        echo "Deleting orphaned tag: $item (created on $date)"
    fi
    gh api -X DELETE "repos/$REPO/git/refs/tags/$item"
done

echo "Deletion complete. All but the last 2 releases/tags (by creation date) have been removed."