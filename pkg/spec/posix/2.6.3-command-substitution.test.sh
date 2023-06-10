#### Command substitution with $()
printf 'output=%s\n' $(echo foo)
## stdout: output=foo

#### Command substitution with ``
printf 'output=%s\n' `echo foo`
## stdout: output=foo

# TODO: Test nested backquotes

# TODO: Test subshell behavior

#### All trailing newlines are removed
# Note: We need to use double quotes here to avoid the interference of field
# splitting.
printf 'output=%s\n' "$(printf 'foo\n\n\n')"
## stdout: output=foo

#### Non-trailing newlines are preserved
printf 'output=%s\n' "$(printf '\nfoo\n\nbar')"
## STDOUT:
output=
foo

bar
## END

# TODO: Test field splitting

# TODO: Test pathname expansion

# TODO: Test ambiguous $(( ))
