#### Parameter expansion inside arithmetic expression
foo=1
echo $(($foo+1))
## stdout: 2

#### Command substitution inside arithmetic expression
echo $((`printf %s%s 1 2`+1))
## stdout: 13

# TODO: Test quote removal

#### Literals
echo $(( 1234 )) $(( 0777 )) $(( 0xbeef )) $(( 0XBEEF ))
## stdout: 1234 511 48879 48879

#### Operator ( )
echo $(( (12) ))
## stdout: 12

#### Operator unary + -
echo $(( + 12 )) $(( - 12 )) 
## stdout: 12 -12

#### Operator ~ !
echo $(( ~0 )) $(( !0 )) $(( !1 )) $(( !1234 ))
## stdout: -1 1 0 0

#### Operator * / %
echo $(( 3 * 4 )) $(( 7 / 3 )) $(( 7 % 3 ))
## stdout: 12 2 1

#### Operator + -
echo $(( 3 + 4 )) $(( 3 - 4 ))
## stdout: 7 -1

#### Operator << >>
echo $(( 15 << 2 )) $(( 15 >> 2 ))
## stdout: 60 3

#### Operator < <= > >=
echo $(( 3 <  2 )) $(( 3 <  3 )) $(( 3 <  4))
echo $(( 3 <= 2 )) $(( 3 <= 3 )) $(( 3 <= 4 ))
echo $(( 3 >  2 )) $(( 3 >  3 )) $(( 3 >  4))
echo $(( 3 >= 2 )) $(( 3 >= 3 )) $(( 3 >= 4 ))
## STDOUT:
0 0 1
0 1 1
1 0 0
1 1 0
## END

#### Operator == !=
echo $(( 2 == 2 )) $(( 2 == 3 )) $(( 2 != 2 )) $(( 2 == 2 ))
## stdout: 1 0 0 1

#### Operator &
echo $(( 11 & 6 )) # 1011 & 0110 = 0010
## stdout: 2

#### Operator ^
echo $(( 11 ^ 6 )) # 1011 ^ 0110 = 1101
## stdout: 13

#### Operator |
echo $(( 11 | 6 )) # 1011 | 0110 = 1111
## stdout: 15

#### Operator &&
echo $(( 0 && 0 )) $(( 1 && 0 )) $(( 0 && 1 )) $(( 1 && 1))
## stdout: 0 0 0 1

#### Operator ||
echo $(( 0 || 0 )) $(( 1 || 0 )) $(( 0 || 1 )) $(( 1 || 1))
## stdout: 0 1 1 1

#### Operator ?:
echo $(( 0 ? 12 : 34 )) $(( 1 ? 12 : 34 ))
## stdout: 34 12

#### Operator =
x=1
echo $(( x = 2 ))
echo $x
## STDOUT:
2
2
## END

#### Operator augmented assignment
x=1
echo $(( x += 10 ))
echo $x
## STDOUT:
11
11
## END

# TODO: Test operator precedence

#### Changes to variables in parameter expansion
x=
echo $(( ${x:=2} )) $x
## stdout: 2 2

#### Variable name
x=+0x10
echo $(( x ))
x=-0x10
echo $(( x ))
## STDOUT:
16
-16
## END

# TODO: Add official example
