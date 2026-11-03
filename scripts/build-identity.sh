#!/usr/bin/env bash
# Source this helper before generating build outputs. Release identity is explicit.
locas_resolve_build_identity() {
  local repo="${1:?repository required}" release_tag="${2-}" head tag_commit dirty
  local component significant max_component=9223372036854775807 LC_ALL=C
  locas_build_version=dev
  locas_build_commit=unknown
  if ! head="$(git -C "$repo" rev-parse --verify HEAD 2>/dev/null)" ||
      [[ ! "$head" =~ ^[0-9a-f]{40}$ ]]; then
    if [[ -n "$release_tag" ]]; then
      echo 'Release identity requires a Git commit' >&2
      return 1
    fi
    export locas_build_version locas_build_commit
    return 0
  fi
  locas_build_commit="$head"
  locas_build_version="dev-${head:0:12}"
  if [[ -n "$release_tag" ]]; then
    # The root-owned installed-version record allows 64 bytes with its newline.
    [[ ${#release_tag} -le 63 ]] || {
      echo 'Release tag exceeds 63-byte installed-version limit' >&2; return 1;
    }
    [[ "$release_tag" =~ ^v([0-9]+)\.([0-9]+)\.([0-9]+)$ ]] || {
      echo 'Malformed release tag' >&2; return 1;
    }
    # Match Go strconv.Atoi on the linux/amd64 build target without shell
    # integer conversion (which could overflow or interpret leading zeros).
    for component in "${BASH_REMATCH[@]:1}"; do
      significant="${component#"${component%%[!0]*}"}"
      significant="${significant:-0}"
      if [[ ${#significant} -gt ${#max_component} ]] ||
          { [[ ${#significant} -eq ${#max_component} ]] &&
            [[ "$significant" > "$max_component" ]]; }; then
        echo 'Release component exceeds linux/amd64 Go int range' >&2
        return 1
      fi
    done
    tag_commit="$(git -C "$repo" rev-parse --verify "refs/tags/${release_tag}^{commit}" 2>/dev/null)" || {
      echo 'Release tag does not resolve to a commit' >&2; return 1;
    }
    [[ "$tag_commit" == "$head" ]] || {
      echo 'Release tag does not match HEAD' >&2; return 1;
    }
    dirty="$(git -C "$repo" status --porcelain --untracked-files=all)" || return 1
    [[ -z "$dirty" ]] || {
      echo 'Release builds require clean tracked and untracked source' >&2; return 1;
    }
    locas_build_version="$release_tag"
  fi
  export locas_build_version locas_build_commit
}
