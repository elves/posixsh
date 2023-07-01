# The effect of set -o nounset is tested in 2.6.2-parameter-expansion.test.sh.

#### set -u is equivalent to set -o nounset
set -u
echo $x
## status: [1, 127]
## stderr-regexp: .+
