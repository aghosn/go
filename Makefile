NAME=gosb

all: $(NAME)

$(NAME):
	sh clean.sh
	sh compile.sh
	sh install.sh

.PHONY: clean

clean:
	@sh clean.sh
	@rm -f gosb_run.sh
