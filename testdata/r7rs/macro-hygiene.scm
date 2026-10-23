; Independently authored check of R7RS-small section 4.3.2.
(import (scheme base))
(write-string (if (let ((tmp 9)) (let-syntax ((keep (syntax-rules () ((_ x) (let ((tmp 0)) x))))) (= (keep tmp) 9))) "ok" "fail"))
