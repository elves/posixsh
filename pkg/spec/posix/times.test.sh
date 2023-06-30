#### times
times
## stdout-regexp: .+m.+s .+m.+s\n.+m.+s .+m.+s\n

#### times errors if given any argument
times 1
## status: [1, 127]
## stderr-regexp: .+
