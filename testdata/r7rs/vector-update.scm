; Independently authored check of R7RS-small section 6.8.
(import (scheme base))
(write-string (if (let ((v (vector 4 5))) (vector-set! v 1 9) (= (vector-ref v 1) 9)) "ok" "fail"))
