# Basics of alias substitution is tested in 2.3.1-alias-substitution.test.sh.

#### alias with no arguments lists aliases
unalias -a
alias foo=bar
alias lorem=ipsum
alias | sort
## STDOUT:
foo=bar
lorem=ipsum
## END

#### alias with just name displays the definition
alias foo=bar
alias foo
## stdout: foo=bar

#### alias with just name errors when alias is not defined
unalias -a
alias foo
## status: [1, 127]
## stderr-regexp: .+
