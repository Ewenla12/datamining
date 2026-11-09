// Fictional student dataset generator for CULCYB 371 (Data Mining & Warehousing Security)
package main

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

const schema = `
DROP TABLE IF EXISTS students;
CREATE TABLE students (
    matric_no        VARCHAR(20) PRIMARY KEY,
    department       VARCHAR(60) NOT NULL,
    level            SMALLINT    NOT NULL CHECK (level IN (100,200,300,400)),
    first_name       VARCHAR(50) NOT NULL,
    last_name        VARCHAR(50) NOT NULL,
    other_names      VARCHAR(80),
    gender           CHAR(1)     NOT NULL CHECK (gender IN ('M','F')),
    date_of_birth    DATE        NOT NULL,
    email            VARCHAR(100) UNIQUE NOT NULL,
    phone            VARCHAR(15),
    state_of_origin  VARCHAR(30) NOT NULL,
    enrollment_year  SMALLINT    NOT NULL,
    attendance_pct   NUMERIC(5,2) NOT NULL CHECK (attendance_pct BETWEEN 0 AND 100),
    avg_exam_score   NUMERIC(5,2) NOT NULL CHECK (avg_exam_score BETWEEN 0 AND 100),
    cgpa             NUMERIC(3,2) NOT NULL CHECK (cgpa BETWEEN 0 AND 5),
    scholarship      BOOLEAN     NOT NULL DEFAULT FALSE,
    status           VARCHAR(12) NOT NULL DEFAULT 'active'
);
CREATE INDEX idx_students_dept_level ON students(department, level);
`

type dept struct{ name, code string }

var (
	depts = []dept{
		{"Computer Science", "CSC"},
		{"Cybersecurity", "CYB"},
		{"Software Engineering", "SEN"},
		{"Information Technology", "IFT"},
		{"Data Science", "DTS"},
	}
	maleNames   = []string{"Chinedu", "Tunde", "Emeka", "Ibrahim", "Segun", "Obinna", "Yusuf", "Femi", "Kelechi", "Daniel", "Abiola", "Musa"}
	femaleNames = []string{"Ngozi", "Aisha", "Folake", "Chiamaka", "Blessing", "Zainab", "Temitope", "Amaka", "Funmi", "Hauwa", "Ifeoma", "Sade"}
	lastNames   = []string{"Okafor", "Adeyemi", "Bello", "Eze", "Balogun", "Nwosu", "Abdullahi", "Okonkwo", "Ogunleye", "Lawal", "Chukwu", "Afolabi", "Danjuma", "Ibe"}
	otherNames  = []string{"Chukwuemeka", "Oluwaseun", "Ayomide", "Kehinde", "Oluwatobi", "Ugochukwu", "Adebayo", "Nnamdi"}
	states      = []string{"Lagos", "Ogun", "Oyo", "Anambra", "Kano", "Rivers", "Enugu", "Kaduna", "Delta", "Edo", "Ondo", "Abuja FCT"}
	statuses    = []string{"active", "active", "active", "active", "active", "active", "active", "deferred", "probation"}
)

func pick[T any](r *rand.Rand, s []T) T { return s[r.Intn(len(s))] }

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
func round2(v float64) float64        { return math.Round(v*100) / 100 }

func main() {
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL is not set (check your .env file)")
	}
	if os.Getenv("ALLOW_RESET") != "true" {
		log.Fatal("this script drops the students table; set ALLOW_RESET=true to continue")
	}

	total := 300
	if v, err := strconv.Atoi(os.Getenv("STUDENT_COUNT")); err == nil && v > 0 {
		total = v
	}
	seed := int64(42)
	if v, err := strconv.ParseInt(os.Getenv("SEED"), 10, 64); err == nil {
		seed = v
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if _, err := db.Exec(schema); err != nil {
		log.Fatal("schema: ", err)
	}

	r := rand.New(rand.NewSource(seed))
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	stmt, err := tx.Prepare(`INSERT INTO students
		(matric_no, department, level, first_name, last_name, other_names, gender, date_of_birth,
		 email, phone, state_of_origin, enrollment_year, attendance_pct, avg_exam_score, cgpa, scholarship, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`)
	if err != nil {
		log.Fatal(err)
	}

	for i := 1; i <= total; i++ {
		d := pick(r, depts)
		level := (r.Intn(4) + 1) * 100
		enrollYear := 2026 - (level/100 - 1)

		gender := "M"
		first := pick(r, maleNames)
		if r.Intn(2) == 0 {
			gender, first = "F", pick(r, femaleNames)
		}
		var other any // NULL for ~40% of students
		if r.Float64() < 0.6 {
			other = pick(r, otherNames)
		}
		last := pick(r, lastNames)

		ability := r.NormFloat64()
		attendance := clamp(76+11*ability+r.NormFloat64()*6, 35, 100)
		score := clamp(56+11*ability+0.15*(attendance-76)+r.NormFloat64()*6, 15, 98)
		cgpa := clamp(score/20+r.NormFloat64()*0.25, 0.8, 5.0)
		scholar := cgpa >= 4.2 && r.Float64() < 0.6

		age := 17 + (level / 100) + r.Intn(5)
		dob := time.Date(2026-age, time.Month(r.Intn(12)+1), r.Intn(28)+1, 0, 0, 0, 0, time.UTC)

		matric := fmt.Sprintf("CU/%02d/%s/%04d", enrollYear%100, d.code, i)
		email := strings.ToLower(fmt.Sprintf("%s.%s%d@students.example.edu", first, last, i))
		phone := fmt.Sprintf("080%08d", r.Intn(100000000)) // fictional, not real numbers

		_, err := stmt.Exec(matric, d.name, level, first, last, other, gender, dob.Format("2006-01-02"),
			email, phone, pick(r, states), enrollYear, round2(attendance), round2(score),
			round2(cgpa), scholar, pick(r, statuses))
		if err != nil {
			tx.Rollback()
			log.Fatal("insert: ", err)
		}
	}
	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Inserted %d fictional students.\n\n", total)

	var corr float64
	db.QueryRow(`SELECT corr(attendance_pct, avg_exam_score) FROM students`).Scan(&corr)
	fmt.Printf("Correlation(attendance, exam score): %.3f\n\n", corr)

	rows, err := db.Query(`SELECT department, COUNT(*), ROUND(AVG(attendance_pct),1), ROUND(AVG(avg_exam_score),1), ROUND(AVG(cgpa),2)
	                       FROM students GROUP BY department ORDER BY department`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	fmt.Printf("%-24s %5s %8s %8s %6s\n", "Department", "N", "Attend", "Score", "CGPA")
	for rows.Next() {
		var name string
		var n int
		var a, s, c float64
		rows.Scan(&name, &n, &a, &s, &c)
		fmt.Printf("%-24s %5d %8.1f %8.1f %6.2f\n", name, n, a, s, c)
	}
}
