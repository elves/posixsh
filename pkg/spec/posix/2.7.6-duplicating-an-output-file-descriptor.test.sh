#### Duplicating standard output from another FD
echo content 3>file >&3
cat file
## stdout: content

#### Duplicating output FD from another FD
echo3() {
    echo "$@" >&3
}
echo3 content 4>file 3>&4
cat file
## stdout: content

# TODO: Test effect of closing FD
