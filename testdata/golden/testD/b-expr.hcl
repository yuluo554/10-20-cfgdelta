resource "thing" "b" {
  count = var.nodes
  name  = "synth"
}
