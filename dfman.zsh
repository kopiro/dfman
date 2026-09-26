# Source from an interactive .zshrc; only reads status before each prompt.
[[ -o interactive ]] || return 0
_dfman_prompt_notice() {
    [[ -t 1 ]] || return 0
    (( $+commands[dfman] )) || return 0
    local notice fingerprint
    notice=$(command dfman status --prompt 2>/dev/null) || return 0
    if [[ -z "$notice" ]]; then
        _dfman_seen_notice=''
        return 0
    fi
    fingerprint=${notice%%$'\n'*}
    if [[ "$fingerprint" != "${_dfman_seen_notice:-}" ]]; then
        print -r -- "${notice#*$'\n'}"
        _dfman_seen_notice=$fingerprint
    fi
    return 0
}
autoload -Uz add-zsh-hook
add-zsh-hook precmd _dfman_prompt_notice
