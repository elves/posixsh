# Most of the tests for the effect of set -o nounset are in
# 2.6.2-parameter-expansion.test.sh.

#### set -o nounset also affects arithmetic expansion
set -o nounset
echo $(( x ))
## status: [1, 127]
## stderr-regexp: .+

#### set -u is equivalent to set -o nounset
set -u
echo $x
## status: [1, 127]
## stderr-regexp: .+
