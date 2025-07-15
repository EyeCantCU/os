import pyghidra

print(dir(pyghidra))

# This also ensures that the .jars in the patch directory work
pyghidra.start()
