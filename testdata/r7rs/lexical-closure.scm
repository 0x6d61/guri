; Independently authored check of R7RS-small section 4.1.4.
(import (scheme base))
(write-string (if (= (((lambda (n) (lambda (x) (+ n x))) 13) 8) 21) "ok" "fail"))
