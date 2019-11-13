# Description
 Visma Verifications divides a Visma cvs verification into multiple xlsx or csv files.
 
 ## Perquisites
 Go 13 must be installed, see [golang.org](https://golang.org) for more information.
 
 ## Usage
 Start by exporting a visma verification list, checking the "Utskott/Kommittéer" checkbox.
 Then copy the file to the `visma-verifications` and run the following to split the file 
 `NAME_OF_FILE.csv` and export csv files into a directory called `export`.
 ```shell script
go run main.go --format csv NAME_OF_FILE.csv export
```