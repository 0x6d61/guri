; Independently authored check of R7RS-small section 4.2.2.
(import (scheme base))
(write-string (if (= (let ((x 3)) (let ((x 8) (y x)) (+ x y))) 11) "ok" "fail"))
