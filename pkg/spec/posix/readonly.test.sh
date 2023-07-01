#### Making a variable readonly makes it a fatal error to assign to it
readonly foo=bar
foo=baz
## status: [1, 127]
## stderr-regexp: .+

#### Making multiple variables readonly
readonly foo=bar lorem=ipsum
lorem=dolar
## status: [1, 127]
## stderr-regexp: .+

# More tests about assigning to a readonly variable are found in
# 2.8.1-consequences-of-shell-errors.test.sh.

#### Making a variable readonly without assigning to it
foo=bar
readonly foo
echo $foo
## stdout: bar

#### Making an unset variable readonly doesn't set it
readonly foo
echo ${foo-unset}
## stdout: unset

#### readonly -p
readonly foo=bar
readonly -p
## stdout-regexp: (?m).*^readonly foo=bar$.*

#### readonly -p, readonly but unset variable
readonly foo
readonly -p
## stdout-regexp: (?m).*^readonly foo$.*
