; Independently authored check of R7RS-small section 4.1.4.
(import (scheme base))
(write-string (if (equal? ((lambda (x . xs) xs) 1 2 3) '(2 3)) "ok" "fail"))
