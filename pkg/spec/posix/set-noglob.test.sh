# The effect of the noglob option is tested in 2.6.6-pathname-expansion.test.sh.

#### set -f is equivalent to set -o noglob
touch foo bar
set -f
printf ': %s\n' *
## stdout: : *
