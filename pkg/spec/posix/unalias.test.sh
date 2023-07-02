#### unalias removes an alias
alias echo='echo foo'
echo bar
unalias echo
echo bar
## STDOUT:
foo bar
bar
## END

#### unalias errors if the alias is not defined
unalias -a
unalias foo
## status: [1, 127]
## stderr-regexp: .+

#### unalias -a removes all aliases
alias foo=bar
alias lorem=ipsum
unalias -a
alias
## stdout-json: ""
