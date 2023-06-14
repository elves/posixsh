#### Creating and appending to file
echo line1 >> file
echo line2 >> file
cat file
## STDOUT:
line1
line2
## END

#### Redirecting a different FD
echo3() {
    echo "$@" >&3
}
echo3 line1 3>> file
echo3 line2 3>> file
cat file
## STDOUT:
line1
line2
## END
