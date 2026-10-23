; Independently authored check of R7RS-small section 6.3.
(import (scheme base))
(write-string (if (and (not #f) (if '() #t #f) (if 0 #t #f) (if "" #t #f)) "ok" "fail"))
