# Compatibility entry point; new installs use dfman shell install.
[[ -o interactive ]] || return 0
if command -v dfman >/dev/null 2>&1; then
  eval "$(dfman shell init zsh)"
fi
