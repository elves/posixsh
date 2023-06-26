#### return aborts function
f() {
    echo a
    return
    echo b
}
f
## stdout: a

#### The argument to return is the exit status of the function
f() {
    echo a
    return 12
    echo b
}
f
## stdout: a
## status: 12

#### Return with no argument uses last pipeline status
f() {
    false
    return
    echo b
}
f
## status: 1

# TODO: return in trap

#### Return raises fatal error when given invalid argument
f() {
    return -1
}
f
echo should not get here
false
## status: [1, 127]
## stdout-json: ""

#### Return raises fatal error when given superfluous arguments
f() {
    return 1 10
}
f
echo should not get here
## status: [1, 127]
## stdout-json: ""
