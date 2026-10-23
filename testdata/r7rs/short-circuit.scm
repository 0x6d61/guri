; Independently authored check of R7RS-small section 4.2.1.
(import (scheme base))
(write-string (if (and (not (and #f (error "unreachable"))) (or #t (error "unreachable"))) "ok" "fail"))
