#### Redirecting standard input to heredoc
cat <<EOF
line1
line2
EOF
## STDOUT:
line1
line2
## END

#### Redirecting a different FD to heredoc
cat3() {
    cat < &3
}
cat3 3<<EOF
line1
line2
EOF
## STDOUT:
line1
line2
## END

#### Heredoc delimiter must appear on its own line
cat <<EOF
line1
line2
 EOF
EOF 
EOF
## STDOUT:
line1
line2
 EOF
EOF 
## END

#### Multiple heredocs
cat03() {
    echo 'fd 0'
    cat
    echo 'fd 3'
    cat < &3
}
cat03 <<EOF0 3<<EOF3
line1
line2
EOF0
more1
more2
EOF3
## STDOUT:
fd 0
line1
line2
fd 3
more1
more2
## END

# TODO:
# #### Expansions when starting word is unquoted
# x=variable
# cat <<EOF
# $x $(echo foo) `echo bar` $(( 7*6 ))
# EOF
# ## STDOUT:
# variable foo bar 42
# ## END

#### No expansion when starting word is quoted
x=variable
cat <<'EOF'
$x $(echo foo) `echo bar` $(( 7*6 ))
EOF
cat <<"EOF"
$x $(echo foo) `echo bar` $(( 7*6 ))
EOF
## STDOUT:
$x $(echo foo) `echo bar` $(( 7*6 ))
$x $(echo foo) `echo bar` $(( 7*6 ))
## END

#### No expansion when starting word is partially quoted
x=variable
cat <<E'OF'
$x $(echo foo) `echo bar` $(( 7*6 ))
EOF
cat <<E"OF"
$x $(echo foo) `echo bar` $(( 7*6 ))
EOF
## STDOUT:
$x $(echo foo) `echo bar` $(( 7*6 ))
$x $(echo foo) `echo bar` $(( 7*6 ))
## END

# TODO:
# #### Stripping leading tabs with <<-
# cat <<-EOF
# 	line1
# 	line2
# 	EOF
# ## STDOUT:
# line1
# line2
# ## EOF
