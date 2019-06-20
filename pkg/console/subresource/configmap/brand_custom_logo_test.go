package configmap

import (
	"testing"

	v1 "github.com/openshift/api/config/v1"

	operator "github.com/openshift/api/operator/v1"

	"github.com/go-test/deep"
)

func TestOnlyFileOrKeySet(t *testing.T) {
	tests := []struct {
		name   string
		input  *operator.Console
		output bool
	}{
		{
			name:   "No custom logo file or key set",
			input:  &operator.Console{},
			output: false,
		}, {
			name: "Both custom logo file and key set",
			input: &operator.Console{
				Spec: operator.ConsoleSpec{
					Customization: operator.ConsoleCustomization{
						CustomLogoFile: v1.ConfigMapFileReference{
							Name: "custom-logo-file",
							Key:  "img.png",
						},
					},
				},
			},
			output: false,
		}, {
			name: "Custom logo file set but not key",
			input: &operator.Console{
				Spec: operator.ConsoleSpec{
					Customization: operator.ConsoleCustomization{
						CustomLogoFile: v1.ConfigMapFileReference{
							Name: "custom-logo-file",
						},
					},
				},
			},
			output: true,
		}, {
			name: "Custom logo key set but not file",
			input: &operator.Console{
				Spec: operator.ConsoleSpec{
					Customization: operator.ConsoleCustomization{
						CustomLogoFile: v1.ConfigMapFileReference{
							Key: "img.png",
						},
					},
				},
			},
			output: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(FileNameOrKeyInconsistentlySet(tt.input), tt.output); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestFileNameNotSet(t *testing.T) {
	tests := []struct {
		name   string
		input  *operator.Console
		output bool
	}{
		{
			name:   "No custom logo file data",
			input:  &operator.Console{},
			output: true,
		}, {
			name: "Custom logo name and key set",
			input: &operator.Console{
				Spec: operator.ConsoleSpec{
					Customization: operator.ConsoleCustomization{
						CustomLogoFile: v1.ConfigMapFileReference{
							Name: "custom-logo-file",
							Key:  "img.png",
						},
					},
				},
			},
			output: false,
		}, {
			name: "Custom logo name set",
			input: &operator.Console{
				Spec: operator.ConsoleSpec{
					Customization: operator.ConsoleCustomization{
						CustomLogoFile: v1.ConfigMapFileReference{
							Name: "custom-logo-file",
						},
					},
				},
			},
			output: false,
		}, {
			name: "Custom logo key set",
			input: &operator.Console{
				Spec: operator.ConsoleSpec{
					Customization: operator.ConsoleCustomization{
						CustomLogoFile: v1.ConfigMapFileReference{
							Key: "img.png",
						},
					},
				},
			},
			output: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(FileNameNotSet(tt.input), tt.output); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestIsLikelyCommonImageFormat(t *testing.T) {
	tests := []struct {
		name   string
		input  []byte
		output bool
	}{
		// using data generated from passing a png,jpg and a gif to `oc create configmap --from-file`
		{
			name:   "PNG is a common image",
			input:  []byte("iVBORw0KGgoAAAANSUhEUgAAAGQAAABkCAYAAABw4pVUAAADmklEQVR4Xu2bv0tyURzGv1KLDoUgLoIShEODDbaHf0JbuNfSkIlR2NAf4NTgoJtQkoODq4OWlVu0NUWzoERQpmDhywm6vJh6L3bu5SmeM8bx3Od+Pvfx/jLX9vb2UDhgCLgoBMbFZxAKwfJBIWA+KIRC0AiA5eE5hELACIDFYUMoBIwAWBw2hELACIDFYUMoBIwAWBw2hELACIDFYUMoBIwAWBw2hELACIDFYUMoBIwAWBw25C8I8Xg8srCwYOxKp9OR9/d3LbvmdrtlcXHRlrW1BLR5kZkacnR0JMFg0IhWq9WkVCppiXp4eChLS0vGWpeXl1IsFrWs/RsW0SKkXq/L+fm5lv0dFXJ1dSWnp6da1v4Ni1AImCUKoZDpBKLRqKysrBiTbm5u5PHxEQybfXHgGmLfrv6OlSkEzJN2IaFQSFZXVyUcDovX65XX11d5enoSdbV0f39vuvt+v18CgYAx7+HhQV5eXkw/91cmaBNyfX0tqVRK1I3dpPH29ibZbFYU5EmDl70z/MPO6I1hu90Wn88nLpfL9EAdDoeSy+Xk7u5u7FwK0SBkHNmPjw+Zm5sbC30wGEgikRj7uIVCNApRR//FxYVUKhXp9XoyPz8vsVhMNjY2vskpFArSbDa/CaMQTUKmfRWp517qa+7/oU7wJycnFDJCQMtJXa2pHi6qh4yTRjqdFnUF9jVarZYcHx9TiB1C+v2+7O7uTj2hb25ufn59fY3n52c5ODigEDuEqMvYTCYzVcj6+rrE43FjTrfblWQySSF2CLm9vZV8Pj9VyNrammxtbRlz1D3J3t4ehdghpNFoyNnZGYWY3oWZT9ByUrfygooNMZehZlCINU6OzaIQx1Bb2xCFWOPk2CwKcQy1tQ1RiDVOjs2iEMdQW9sQhVjj5NgsCnEMtbUNUYg1To7NmknI6EukarUq5XJ5auhIJCI7OzvGHPXDBfUOfnTs7+/L8vKy8Wedvxt2jOoPNjSTkB9sjx81IUAhYIcIhVAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhlAIGAGwOGwIhYARAIvDhoAJ+QeTS82niTWiVwAAAABJRU5ErkJggg=="),
			output: true,
		}, {
			name:   "JGEG is a common image",
			input:  []byte("/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAAEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQEBAQH/wAALCABkAGQBAREA/8QAGwABAQADAQEBAAAAAAAAAAAAAAkGBwgDBAL/xAAxEAAABwACAQIDBgYDAAAAAAAAAQIDBAUGBwgJESESd7YKFDE2OHYTFSI3QbUjObf/2gAIAQEAAD8A4vAAAAAAAAAAAAAAAAAB7xosqY6TESM/KeMlKJmMy4+6aUl6qUTbSVLMkl7qMi9CL3MebrTrDi2Xm3GXmlqbcadQptxtaTMlIWhZEpC0mRkpKiIyMjIyIx+B9UWDNnLUiDDlTFoT8a0RY70haEGfoSlJZQs0p9TIviMiL1P09fUfKZGRmRkZGR+hkfsZGX4kZf4MgAAAAAWt8C362ND8hN19WceDnDyF4HdcieQvs7Scf4rW7m5LepdOox2cuNNZk0dBSl/FOBSQ5somzMjL4za+H1I/f1IxwFqMjrMPcP57a5jQ5C/ioack0eopbLP3Edt5PxsuP1ltGiTWUOp/qaU4wlLifdBmXuKf+LDu9S9MdDzJMueKt9ygjkKmxkZhjAtxH5dMvMTtC6t2wYlkRHGmlepQ06hxJtuxjQaFk8Sm5p8kaZva8ib3ZNQXqxrW7TU6ZqtkLS6/Xt315OtUQX3EobS49ETLJh1aW0JWttSiQkjJJYWAAAAAtb4Fv1saH5Cbr6s48FEO/vlie6ec1afg3rXxPxvO2FfaQtRzLrdnUWi6i11unoquzTGjV2Tu8rYXV63QPUCLTU215JNlLTWcZrfgqEPtZ1zfqOP/ACXeKbadgNHhajNch8dZDebGtcjqTazsRsuLHnbHSQ6C5eYjTU0O5ztMgplZKJxtqBdwilffrSir7VPJf2d3819qf29xJ/suQRBjsB/fjmz5uckfWV0NRgAAAALW+Bb9bGh+Qm6+rOPByL5PlrX357NqWo1GW/aQRqMzMkN5yibbT7/4QhKUJL8CSkiL2IWb6D/9J3bT9q9q/wDypga0+zu/mvtT+3uJP9lyCIMdgP78c2fNzkj6yuhqMAAAABQzxndtOOemXYe05b5QpdrfZudxlpMW1CwVdRWl2m0uLvLWUWQ5G0Ojy8AoDbFHKQ+6myVIQ65HJuK6hTi2tE9xeZcx2E7Ncxc0YyBfVeX5B1P87pYGnjV8O/jRCrK+CTdpFqrO5rmJJuQ3FmiJaTWiQpH/ADGo1JTQLrJ5EuFuF/Hhzj1K1OW5Rn8j8l03NVbQXdBTZOViYrvJGKazlI5b2Njtaq9jtxJ6Vu2v3LN2CmYhJciJmvKOOnEfFV3z4h6N3fNNlyzm+SNDH5HqsNBpE8d1GYtnoj2Zl6h+cq1TpdhkkMNPIu4pRFRHJylrbkE8hgktqdmdyfp4O25K5D2dWzLj1mu3Ot09dHnoZbnMQb6/sLWIzNbjvyo7ctqPLbRJQxJkMpeStLT7qCS4rBgAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAB//9k="),
			output: true,
		}, {
			name:   "GIF is a common image",
			input:  []byte("R0lGODlhZABkAPUyAGZmZmpqam9vb3BwcHNzc3x8fH5+foKCgoSEhIWFhYiIiIqKipCQkJSUlJeXl5mZmZqamqCgoKWlpaioqKurq7S0tLm5ub29vb6+vsDAwMHBwcLCwsPDw8zMzM3NzdXV1dra2tvb297e3t/f3+Dg4OHh4efn5+rq6uvr6+/v7/Ly8vT09PX19fb29vf39/r6+vz8/P7+/v///wAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAACH5BAAAAAAALAAAAABkAGQAAAb+QIBwSCwaj8ikcslsOp/QqHRKrVqv2Kx2y+16v+CweEwum8/otHrNbrvf8Lh8Tq/b7/i8fs/v+/+AgYKDYwMGhwFSAocGiYRDJTKSF1IikjIZj5CXGJWXG5pCkZKdUZaSoKGjMqVQpzKpmqutTw8ctwqhALO6V7y9Vb8JFSAoIx4MSQUOzAS6swctl9MsC0evsY+rJzHT3jEORtjP3pcw5S+OQ+Oq3xgCAAEU55cRReyy0+BFCNMf95/IXZpwhMQlEwBRCZThAgmGSykSwloYAomESyskZiO0qgMSCJdYaFyo4WPIke1ImZQkkgg+bZxWymi5LmBKVjJpCnnJMeZyEZAsUeZT+fOkS5tDcRYNelThTVpEgM4UCpOoEak6AfAc9Ouq0ZpOk0IdgpUqoVcWkDS4pKJIiEuUgMmdS7eu3bt48+rdy7ev37+AAwseTLiw4cOIEytezLix48eQI0ueTLmy5cuYM2vezLmz58+g6wYBADs="),
			output: true,
		}, {
			name:   "Random noise is not an image",
			input:  []byte("&^%$#&*"),
			output: true,
		}, {
			name:   "Plain text is not an image",
			input:  []byte("this is not an image"),
			output: true,
		},
		// TODO: include some other file types as binary imput
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(IsLikelyCommonImageFormat(tt.input), tt.output); diff != nil {
				t.Error(diff)
			}
		})
	}
}

func TestLogoImageIsEmpty(t *testing.T) {
	tests := []struct {
		name   string
		input  []byte
		output bool
	}{
		{
			name:   "Byte array has data",
			input:  []byte("data that would represent an image"),
			output: false,
		}, {
			name:   "Byte array is empty",
			input:  []byte(""),
			output: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if diff := deep.Equal(LogoImageIsEmpty(tt.input), tt.output); diff != nil {
				t.Error(diff)
			}
		})
	}
}
