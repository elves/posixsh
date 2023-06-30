#### exit exits
f() {
    g
}
g() {
    exit 10
}
while true; do
    f
done
echo should not get here
## status: 10

#### exit stops at subshell boundary
(exit 10)
echo $?
## stdout: 10

# TODO: Test behavior of exit within trap
