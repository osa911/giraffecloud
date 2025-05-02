variable "db_url" {
  type = string
}

variable "db_dev_url" {
  type = string
}

env "dev" {
  url = var.db_url
  dev = var.db_dev_url
  src = "ent://internal/db/ent/schema"
  migration {
    dir = "file://internal/db/migrations"
  }
}

env "prod" {
  url = var.db_url
  dev = var.db_dev_url
  src = "ent://internal/db/ent/schema"
  migration {
    dir = "file://internal/db/migrations"
  }
}
