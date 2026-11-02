(import (scheme base) (scheme file) (scheme process-context)
        (scheme write) (guri filesystem) (guri process))

;; Search real directories only, avoiding cycles through directory symlinks.
(define (visit path)
  (if (file-exists? (string-append path "/.git"))
      (call-with-values
        (lambda ()
          (run-process '("git" "status" "--short")
                       (list (cons 'directory path))))
        (lambda (status output diagnostic)
          (display path)
          (newline)
          (display (utf8->string output))
          (unless (zero-status? status)
            (display (utf8->string diagnostic) (current-error-port))
            (error "git status failed" path status))))
      (for-each
        (lambda (name)
          (let ((child (string-append path "/" name)))
            (when (file-directory? child #f) (visit child))))
        (directory-files path #t))))

(define (zero-status? status) (and (integer? status) (= status 0)))
(define args (cdr (command-line)))
(visit (if (null? args) "." (car args)))
