# The effect of noclobber is tested in 2.7.2-redirecting-output.test.sh

#### set -C is equivalent to set -o noclobber
echo old > file
set -C
echo new > file
## status: [1, 127]
## stderr-regexp: .+
