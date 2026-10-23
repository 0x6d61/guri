; Independently authored check of R7RS-small section 6.10.
(import (scheme base))
(write-string (if (= (+ 10 (call/cc (lambda (k) (+ 100 (k 7))))) 17) "ok" "fail"))
