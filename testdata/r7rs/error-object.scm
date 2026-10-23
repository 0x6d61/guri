; R7RS-small 6.11: error raises a catchable error object with irritants.
(import (scheme base))
(guard (e ((and (error-object? e)
                (string=? (error-object-message e) "intentional")
                (equal? (error-object-irritants e) '(7)))
           (write-string "ok")))
  (error "intentional" 7))
