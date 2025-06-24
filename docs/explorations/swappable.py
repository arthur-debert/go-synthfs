#3 this example is more contrived since python has no notion of a fs interface
# nor are file system ops in one module

# the use case is this: suppose that you already have aan application with fs 
# code, lik
import os

os.mkdir("code")
# other code


# synth fs gives you a way to swap that 
from synthfs import ops, os


# previous code will work
os.mkdir("code")
# other code


print (len(os.batch.ops()) # these are the ops 
# once your are ready
os.batch.execute()

