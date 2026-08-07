#!/bin/sh

set -eu

archive_path=${1:-}
if [ -z "$archive_path" ] || [ ! -f "$archive_path" ]; then
  echo "Usage: $0 /path/to/extension.zip" >&2
  exit 2
fi

for command_name in curl jq unzip; do
  if ! command -v "$command_name" >/dev/null 2>&1; then
    echo "Required command not found: $command_name" >&2
    exit 2
  fi
done

for variable_name in CWS_ACCESS_TOKEN CWS_PUBLISHER_ID CWS_EXTENSION_ID; do
  eval "variable_value=\${$variable_name:-}"
  if [ -z "$variable_value" ]; then
    echo "Required environment variable is empty: $variable_name" >&2
    exit 2
  fi
done

publish_type=${CWS_PUBLISH_TYPE:-DEFAULT_PUBLISH}
case "$publish_type" in
  DEFAULT_PUBLISH|STAGED_PUBLISH) ;;
  *)
    echo "CWS_PUBLISH_TYPE must be DEFAULT_PUBLISH or STAGED_PUBLISH." >&2
    exit 2
    ;;
esac

manifest_version=$(unzip -p "$archive_path" manifest.json | jq -er '.version')
item_name="publishers/$CWS_PUBLISHER_ID/items/$CWS_EXTENSION_ID"
api_root="https://chromewebstore.googleapis.com/v2/$item_name"
upload_url="https://chromewebstore.googleapis.com/upload/v2/$item_name:upload"
work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT HUP INT TERM

request() {
  output_file=$1
  shift
  if ! curl --fail-with-body --silent --show-error \
    --header "Authorization: Bearer $CWS_ACCESS_TOKEN" \
    --output "$output_file" \
    "$@"; then
    if [ -s "$output_file" ]; then
      jq . "$output_file" >&2 2>/dev/null || cat "$output_file" >&2
    fi
    return 1
  fi
}

status_response="$work_dir/status-before-upload.json"
request "$status_response" --request GET "$api_root:fetchStatus"
if jq -e --arg version "$manifest_version" '
  [
    .publishedItemRevisionStatus.distributionChannels[]?.crxVersion,
    .submittedItemRevisionStatus.distributionChannels[]?.crxVersion
  ] | any(. == $version)
' "$status_response" >/dev/null; then
  echo "Chrome Web Store already has version $manifest_version published or submitted; nothing to do."
  exit 0
fi

echo "Uploading Chrome Web Store package version $manifest_version..."
upload_response="$work_dir/upload.json"
request "$upload_response" \
  --request POST \
  --header "Content-Type: application/zip" \
  --upload-file "$archive_path" \
  "$upload_url"

upload_state=$(jq -er '.uploadState' "$upload_response")
uploaded_version=$(jq -r '.crxVersion // empty' "$upload_response")
if [ -n "$uploaded_version" ] && [ "$uploaded_version" != "$manifest_version" ]; then
  echo "Uploaded version mismatch: expected $manifest_version, received $uploaded_version" >&2
  exit 1
fi

case "$upload_state" in
  SUCCEEDED) ;;
  IN_PROGRESS|UPLOAD_IN_PROGRESS)
    attempt=1
    while [ "$attempt" -le 24 ]; do
      sleep 5
      status_response="$work_dir/status-$attempt.json"
      request "$status_response" --request GET "$api_root:fetchStatus"
      upload_state=$(jq -er '.lastAsyncUploadState' "$status_response")
      case "$upload_state" in
        SUCCEEDED) break ;;
        IN_PROGRESS|UPLOAD_IN_PROGRESS) ;;
        *)
          echo "Chrome Web Store upload failed with state: $upload_state" >&2
          jq . "$status_response" >&2
          exit 1
          ;;
      esac
      attempt=$((attempt + 1))
    done
    if [ "$upload_state" != "SUCCEEDED" ]; then
      echo "Chrome Web Store upload did not finish within two minutes." >&2
      exit 1
    fi
    ;;
  *)
    echo "Chrome Web Store upload failed with state: $upload_state" >&2
    jq . "$upload_response" >&2
    exit 1
    ;;
esac

publish_payload=$(jq -nc \
  --arg publish_type "$publish_type" \
  '{publishType: $publish_type, skipReview: false, blockOnWarnings: true}')
publish_response="$work_dir/publish.json"
echo "Submitting version $manifest_version for Chrome Web Store review..."
request "$publish_response" \
  --request POST \
  --header "Content-Type: application/json" \
  --data "$publish_payload" \
  "$api_root:publish"

submission_state=$(jq -er '.state' "$publish_response")
warning_count=$(jq -r '.warningInfo.warnings | length // 0' "$publish_response")
if [ "$warning_count" -ne 0 ]; then
  echo "Chrome Web Store returned unexpected warnings." >&2
  jq '.warningInfo' "$publish_response" >&2
  exit 1
fi

echo "Chrome Web Store submission accepted: item=$CWS_EXTENSION_ID version=$manifest_version state=$submission_state"
