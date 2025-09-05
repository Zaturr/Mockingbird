#TASKS

Project description:
Route at /models describes how the gin server must behave. The servers are mock servers configured by user.
As you can see there's a list of http servers that might instantiate and must create endpoints based 
on the Location struct. There is a ChaosInjection that might or might not apply. I need you to do the following tasks.

1. Make sure the project directory structure is correct and adapts to the go standard.
2. Make sure the modelo.go correctly adapts to the dynamic needs.
3. Modify the folders and go file names according to a more standard structure as well as the packages. Also take in account that
the project must be called Catalyst and not mockingbird. Please refactor everything as needed. Do not take the current implementation
as holding in order for you to refactor.
4. Correctly load the .yaml configuration based on the model.
5. Dynamically create the servers.
6. Dynamically create the handlers (I was thinking in the use of generics Which i don't know if perfectly adapts to this)
7. In the case Schema is set for the location. Please use the library github.com/santhosh-tekuri/jsonschema/v6 v6.0.1 and validate the schema that is proportioned.
8. Make the proper tests for a robust code and make it simple and maintainable.