package utils

import (
	"fmt"
	"strconv"
	"strings"
)

func Portrane(s string) ([]int, error) {
	o := make([]int, 0)
	dup := map[int]bool{} //去重
	of := make([]int, 0)

	m := strings.Split(s, ",")
	if len(m) > 0 {
		for i := 0; i < len(m); i++ {
			n := strings.Split(m[i], "-")
			if len(n) < 2 {
				if v, e := strconv.Atoi(strings.TrimSpace(m[i])); e == nil {
					if !dup[v] {
						o = append(o, v)
						dup[v] = true
					} else {
						of = append(of, v)
					}
				}
			} else {
				if len(n)%2 == 0 {
					for i := 0; i < len(n); i += 2 {
						nmin, _ := strconv.Atoi(strings.TrimSpace(n[i]))
						nmax, _ := strconv.Atoi(strings.TrimSpace(n[i+1]))
						if nmin != nmax {
							if nmin > nmax {
								nmin = nmax
								nmax, _ = strconv.Atoi(strings.TrimSpace(n[i]))
							}

							for ; nmin <= nmax; nmin++ {
								if !dup[nmin] {
									o = append(o, nmin)
									dup[nmin] = true
								} else {
									of = append(of, nmin)
								}
							}
						} else if nmin != 0 {
							if !dup[nmin] {
								o = append(o, nmin)
								dup[nmin] = true
							} else {
								of = append(of, nmin)
							}
						}
					}
				} else {
					fmt.Println(m[i], "is error")
				}
			}
		}
	} else {
		//仅单个分隔符-
		m = strings.Split(s, "-")
		if (len(m) > 0) && (len(m)%2 == 0) {
			for i := 0; i < len(m); i += 2 {
				nmin, _ := strconv.Atoi(strings.TrimSpace(m[i]))
				nmax, _ := strconv.Atoi(strings.TrimSpace(m[i+1]))
				if nmin != nmax {
					if nmin > nmax {
						nmin = nmax
						nmax, _ = strconv.Atoi(strings.TrimSpace(m[i]))
					}

					for ; nmin <= nmax; nmin++ {
						if !dup[nmin] {
							o = append(o, nmin)
							dup[nmin] = true
						} else {
							of = append(of, nmin)
						}
					}
				} else if nmin != 0 {

					if !dup[nmin] {
						o = append(o, nmin)
						dup[nmin] = true
					} else {
						of = append(of, nmin)
					}
				}
			}
		} else if len(m) == 1 {
			if v, e := strconv.Atoi(strings.TrimSpace(s)); e == nil {
				if !dup[v] {
					o = append(o, v)
					dup[v] = true
				} else {
					of = append(of, v)
				}
			} else {
				return o, fmt.Errorf(s, "is error")
			}
		} else {
			return o, fmt.Errorf(s, "is error")
		}
	}

	//去重
	if len(of) > 0 {
		fmt.Println("dup ports total", len(of), ":\n", of)
	}
	return o, nil
}
