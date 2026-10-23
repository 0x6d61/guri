; Independently authored check of R7RS-small section 6.4.
(import (scheme base))
(write-string (if (let ((p (cons 'a 'b))) (set-cdr! p 'c) (eq? (cdr p) 'c)) "ok" "fail"))
