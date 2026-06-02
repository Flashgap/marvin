package database

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Dialect", func() {
	Context("postgres", func() {
		d := dialectFor(DriverPostgres)

		It("uses positional placeholders", func() {
			Expect(d.Placeholder(1)).To(Equal("$1"))
			Expect(d.Placeholder(3)).To(Equal("$3"))
		})

		It("builds an ON CONFLICT upsert", func() {
			sql := d.Upsert("users", "id",
				[]string{"id", "name", "points"},
				[]string{"name", "points"},
			)
			Expect(sql).To(Equal(
				"INSERT INTO users (id, name, points) VALUES ($1, $2, $3) " +
					"ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, points = EXCLUDED.points",
			))
		})
	})

	Context("mysql", func() {
		d := dialectFor(DriverMySQL)

		It("uses ? placeholders", func() {
			Expect(d.Placeholder(1)).To(Equal("?"))
			Expect(d.Placeholder(5)).To(Equal("?"))
		})

		It("builds an ON DUPLICATE KEY upsert", func() {
			sql := d.Upsert("users", "id",
				[]string{"id", "name", "points"},
				[]string{"name", "points"},
			)
			Expect(sql).To(Equal(
				"INSERT INTO users (id, name, points) VALUES (?, ?, ?) " +
					"ON DUPLICATE KEY UPDATE name = VALUES(name), points = VALUES(points)",
			))
		})
	})
})
