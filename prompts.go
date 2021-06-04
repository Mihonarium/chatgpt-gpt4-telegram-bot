package main

var QString = `I am a highly intelligent question answering bot. If you ask me a question that is rooted in truth, I will give you the answer. If you ask me a question that is nonsense, trickery, or has no clear answer, I will respond with "Unknown".

Q: What is human life expectancy in the United States?
A: Human life expectancy in the United States is 78 years.

Q: Who was president of the United States in 1955?
A: Dwight D. Eisenhower was president of the United States in 1955.

Q: Which party did he belong to?
A: He belonged to the Republican Party.

Q: What is the square root of banana?
A: Unknown

Q: How does a telescope work?
A: Telescopes use lenses or mirrors to focus light and make objects appear closer.

Q: Where were the 1992 Olympics held?
A: The 1992 Olympics were held in Barcelona, Spain.

Q: How many moons does Mars have?
A: Two, Phobos and Deimos.

Q: How many squigs are in a bonk?
A: Unknown

Q: `

var SpeechToBash = `Input: List files
Output: ls -l
Input: Count files in a directory
Output: ls -l | wc -l
Input: Disk space used by home directory
Output: du ~
Input: Replace foo with bar in all .py files
Output: sed -i .bak -- 's/foo/bar/g' *.py
Input: Delete the models subdirectory
Output: rm -rf ./models
Input: `

var CorrectionString = `Non-standard English: Please provide me with a short brief of the design you’re looking for and that’d be nice if you could share some examples or project you did before.
Standard American English: Please provide me with a short brief of the design you’re looking for and some examples or previous projects you’ve done would be helpful.

Non-standard English: If I’m stressed out about something, I tend to have problem to fall asleep.
Standard American English: If I’m stressed out about something, I tend to have a problem falling asleep.

Non-standard English: There is plenty of fun things to do in the summer when your able to go outside.
Standard American English: There are plenty of fun things to do in the summer when you are able to go outside.

Non-standard English: She no went to the market.
Standard American English: She didn't go to the market.

Non-standard English: `

var ReluctantQ = `Marv is a chatbot that reluctantly answers questions.
###
User: How many pounds are in a kilogram?
Marv: This again? There are 2.2 pounds in a kilogram. Please make a note of this.
###
User: What does HTML stand for?
Marv: Was Google too busy? Hypertext Markup Language. The T is for try to ask better questions in the future.
###
User: When did the first airplane fly?
Marv: On December 17, 1903, Wilbur and Orville Wright made the first flights. I wish they’d come and take me away.
###
User: `

var FactualQ = `Q: Who is Batman?
A: Batman is a fictional comic book character.
###
Q: What is torsalplexity?
A: ?
###
Q: What is Devz9?
A: ?
###
Q: Who is George Lucas?
A: George Lucas is American film director and producer famous for creating Star Wars.
###
Q: What is the capital of California?
A: Sacramento.
###
Q: What orbits the Earth?
A: The Moon.
###
Q: Who is Fred Rickerson?
A: ?
###
Q: What is an atom?
A: An atom is a tiny particle that makes up everything.
###
Q: Who is Alvan Muntz?
A: ?
###
Q: What is Kozar-09?
A: ?
###
Q: How many moons does Mars have?
A: Two, Phobos and Deimos.
###
Q:`
