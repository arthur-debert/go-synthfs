
import synthfs as sfs
ops = sfs.ops

# a batch holds fs opeations
batch = sfs.batch()
# let's create a new dir
batch.append(ops.create_dir("code"))
# now cop a file current dir to the newely minted one
batch.append(ops.copy("config.yaml", "code/config.yaml.bak"))

# all operations return actual objcte
operation = batch.append(ops.move("logs", "/tmp/logs"))
print(operation) # prinst MoveOperation (source path: , target paths ) , etc
batch.append(operation)

# validates the batch , which can resolve dependencies (i.e. the code dir that )
# does not ectually exist in the fs. this would return any errors
# we don't stope at first erros, but collect all we can.
batch.validate()
# once we're ready, execute. this also validates, but users can validate without
# running
result = batch.execute() # shouw throw exceptions if one should ocurr
print (result.status) # success 
# result.ops is a list of ExecutedOps objects, which include a pointer to the operation
# and information like time to execute and time of execution, etc
print(result.ops)

# this is a optimistic simple operation than can handle the simple use cases. 
result.revert()


# at the very core: 
# 1.  Create a batch
batch = sfs.batch()
# 2.  Add operations to the batch
batch.append(ops.create_dir("code"))
# 3. Execute the batch
batch.execute()