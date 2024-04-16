# wrtag
complete -c wrtag -f -n __fish_use_subcommand \
    -o dry-run

complete -c wrtag -f -n __fish_use_subcommand -a move
complete -c wrtag -f -n __fish_use_subcommand -a copy

complete -c wrtag -f -n "__fish_seen_subcommand_from copy move" \
    -o keep-file \
    -o path-format \
    -o research-link \
    -o tag-weight \
    -o config-path

# wrtagsync
complete -c wrtagsync -f \
    -o dry-run \
    -o interval \
    -o keep-file \
    -o path-format \
    -o tag-weight \
    -o config-path

# wrtagweb
complete -c wrtagweb -f \
    -o keep-file \
    -o path-format \
    -o tag-weight \
    -o config-path \
    -o listen-addr \
    -o public-url \
    -o api-key \
    -o db-path
