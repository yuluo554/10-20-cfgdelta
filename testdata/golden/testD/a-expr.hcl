resource "thing" "b" {
  count = var.replicas
  name  = "synth"
}
