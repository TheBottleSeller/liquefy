

#Pare file 


with open("nasdaqlisted.txt", "r") as ins:
    array = []
    for line in ins:
        array.append(line)
	items = line.split("|")
	if len(items) > 0:
		print items[0]


