#### Expanding a lone ~
HOME=/home/user
echo ~
## stdout: /home/user

#### Expanding a ~ followed by unquoted /
HOME=/home/user
echo ~/foo
## stdout: /home/user/foo

#### ~ followed by non-bareword before / is seen is not expanded
HOME=/home/user
echo ~$(echo foo)/
## stdout: ~foo/

# TODO: Test ~uname

#### Result of tilde expansion doesn't undergo field splitting or pathname expansion
HOME='/home/  *'
touch foo
printf ': %s\n' ~
## stdout: : /home/  *
