package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"runtime/pprof"
	"sort"
	"strconv"
	"strings"
	"sync"

	"golang.org/x/exp/constraints"
	"golang.org/x/sync/errgroup"
)

func play() {
	target := targets[rand.Intn(len(targets))]
	s := newState(target)

	r := bufio.NewReader(os.Stdin)
	i := 1
	errs := 0
	possible := make([]string, len(valid))
	copy(possible, valid)
	for {
		fmt.Println("Valid words:", len(possible))
		fmt.Printf("Guess %d: ", i)
		guess, err := r.ReadString('\n')
		if err != nil {
			fmt.Println("Read failed:", err)
			errs++
			if errs > 10 {
				break
			}
			continue
		}
		guess = strings.TrimSpace(guess)
		if !validWord(guess) {
			fmt.Println("Invalid guess:", guess)
			continue
		} else if err := s.hardModeProblem(guess); err != nil {
			fmt.Println("Hard mode:", err)
			continue
		}

		result, won := s.guess(guess)
		fmt.Printf("Clues %d: %v\n", i, resultString(result))
		if won {
			fmt.Println("You won!")
			return
		}

		var newPossible []string
		for _, w := range possible {
			if s.hardModeOK(w) {
				newPossible = append(newPossible, w)
			}
		}
		possible = newPossible

		i++
	}
}

func playRandomly(target string, quiet bool) int {
	s := newState(target)
	i := 1
	possible := make([]string, len(valid))
	copy(possible, valid)
	for {
		guess := possible[rand.Intn(len(possible))]
		result, won := s.guess(guess)

		if !quiet {
			fmt.Println("Valid words:", len(possible))
			fmt.Printf("Guess %d: %v\n", i, guess)
			fmt.Printf("Clues %d: %v\n", i, resultString(result))
		}

		if won {
			if !quiet {
				fmt.Println("You won!")
			}
			return i
		}

		var newPossible []string
		for _, w := range possible {
			if s.hardModeOK(w) {
				newPossible = append(newPossible, w)
			}
		}
		possible = newPossible

		i++
	}
}

func playManyRandomly(nWords, nTrials int) {
	var results [100][]string
	var mu sync.Mutex
	var g errgroup.Group
	for i := 0; i < nWords; i++ {
		g.Go(func() error {
			target := targets[rand.Intn(len(targets))]
			for j := 0; j < nTrials; j++ {
				guesses := playRandomly(target, true)
				mu.Lock()
				results[guesses] = append(results[guesses], target)
				mu.Unlock()
			}
			return nil
		})
	}
	g.Wait()

	n := nWords * nTrials
	w := "%" + strconv.Itoa(len(strconv.Itoa(n))) + "d"
	cum := 0
	for i, r := range results {
		cum += len(r)
		if len(r) > n/100 {
			fmt.Printf(w+": "+w+"/"+w+" (cum. "+w+"/"+w+")\n", i, len(r), n, cum, n)
		} else if len(r) > 0 {
			fmt.Printf(w+": "+w+"/"+w+" (cum. "+w+"/"+w+") %s\n", i, len(r), n, cum, n, strings.Join(r, " "))
		}
	}
}

const m = 100

type metricImpl[T constraints.Ordered] struct {
	name        string
	badnessFunc func(*[m]int) T
}

func (m *metricImpl[T]) run(results map[string]*[m]int) string {
	var worst T
	var worstWords []string
	for w, r := range results {
		badness := m.badnessFunc(r)
		switch {
		case worst < badness:
			worstWords = []string{w}
			worst = badness
		case worst == badness:
			worstWords = append(worstWords, w)
		}
	}
	sort.Strings(worstWords)
	return fmt.Sprintf("worst %v: %v (%v)", m.name, worst, strings.Join(worstWords, " "))
}

type metric interface {
	run(results map[string]*[m]int) string
}

var metrics = []metric{
	&metricImpl[int]{"worst", func(r *[m]int) int {
		for i := m - 1; i >= 0; i-- {
			if r[i] > 0 {
				return i
			}
		}
		panic("empty result")
	}},
	&metricImpl[int]{"best", func(r *[m]int) int {
		for i := 0; i < m; i++ {
			if r[i] > 0 {
				return i
			}
		}
		panic("empty result")
	}},
	&metricImpl[float64]{"average", func(r *[m]int) float64 {
		sum := 0
		ct := 0
		for i := 0; i < m; i++ {
			sum += i * r[i]
			ct += r[i]
		}
		return float64(sum) / float64(ct)
	}},
	&metricImpl[float64]{"not-in-6", func(r *[m]int) float64 {
		cutoff := 6
		win := 0
		loss := 0
		for i := 0; i <= cutoff; i++ {
			win += r[i]
		}
		for i := cutoff + 1; i < m; i++ {
			loss += r[i]
		}
		return 100 * float64(loss) / float64(win+loss)
	}},
}

func playAll(nTrials int) {
	nWords := len(targets)
	results := make(map[string]*[m]int, nWords)
	for _, w := range targets {
		results[w] = new([m]int)
	}

	var g errgroup.Group
	for _, target := range targets {
		target := target
		g.Go(func() error {
			for j := 0; j < nTrials; j++ {
				guesses := playRandomly(target, true)
				results[target][guesses]++
			}
			return nil
		})
	}
	g.Wait()

	for _, metric := range metrics {
		fmt.Println(metric.run(results))
	}
}

func main() {
	if true {
		outfile := "profile.pprof"
		f, err := os.Create(outfile)
		if err != nil {
			panic(err)
		}
		defer f.Close()
		err = pprof.StartCPUProfile(f)
		if err != nil {
			panic(err)
		}
		defer pprof.StopCPUProfile()
	}

	// rand.Seed(time.Now().UnixNano())
	// rand.Seed(3)
	// playManyRandomly(1000, 10)
	playAll(1000)
}
