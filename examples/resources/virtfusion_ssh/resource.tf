# SSH keys are scoped to the authenticated account (via the bearer token),
# not a user_id field — and the real API has no update endpoint, so both
# attributes are RequiresReplace: changing either deletes and recreates
# the key.
resource "virtfusion_ssh" "dummy_key" {
  # This is what is displayed in the UI on the SSH keys page.
  name = "dummy_key"

  public_key = "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAACAQCRM5gzj6BpVbTEZ8XX5meQOC9X+znTMCQbXTfdqm9IP3HY2JbqH+yfCBWSsLpXim6WvsYtfkAhrtrkdmaX66Wn1uo6XvARwi/5D1VRTM94vwoitJb0rne4OorpwGIGCpDIi1iRA/ERIbAIQpw/2PJfm7q+fEj9TS+n/MzYOOmwTaKPEJ8+wHwXbjcSNoBQmEPonafbQKQN5PXe5rwnTNAqJWhGPHqF2t7lvZy+m7Sl7X1vUVlw+7iZzOVm9iDXmUInc8A0kz18l/O+4ELhRxxzjmSX5/KkN0GG7wS7CHlq9MS2741MS6p0ZNMgTT/04RfsY5JXoOa1gCeAdnXQST9ylvBd6hXubV95lRM8AXAhEJFHpa0Xn1gHMJ4F0cjjvmBIDx39QztuYsNJPk8veBBQwhOzhnJ3Zh2IYTQD+Mwu5yUrJzUt7ia8X5fhjbrYlfUgdH+siBbvJRzyXwnZdHArher55U4xPCJO4qRrFr72Jn+WGzkcY53oLnW5K3NnPaYViCJD2BgJZU1YF8oA3RyEG+2GS7Ksqs2nXXlZ1c+RXLUXM0pxDrwqvYrE3Ae+O/PtZ0cqpesyjxDfH/R2cj86jjdEi7S8nhgkumHwkoac8LCJnoAeC9S7sxmI99VBHcNwCazx3ZL2UAI3Ik/DQBZXcCPXw9MfY25SyQwEYftMKw== dummy_key"
}