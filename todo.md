- write our system tests - core server frameowrk ex: db, migrations, app provisioning
- write tests for our api
- refactor client - removing and cli functionality 
- build browser group 
- create worker client, pool client, session instance client ( that actually communicates with our browser session ), webhook client that interacts with the instance
- create cli and sdk from client code 
- build 


- link server api and browser instance api. we implemented a server webhook api for defining webhooks and implemented the webhook api in the browser instance. that sends cdp events to set webhooks. 

-figure out tagging, semver, and releases. we will only do this for our server, worker, and client. but we will have multiple clients for different languages.  


-write browser tests
- add readme badges, automated based on test results 
- profiles aka persistent user data dirs
  - these will allow users to define their own profiles in our server and some how we will create a volume for the browser instance to use. managed by our server. 
  - this is hard due to the integrations, we need to support docker, kubernetes, azure container instances, aws, gcp etc. we need to also support multiple browser types and versions. 
  -lots can go wrong here, we need to think about this carefully.   
  - all these systems handle volumes differently, so we need to find a way to abstract this. 
  - the solution must support multiple profiles per instance.  and allow multiple instances to share the same profile.  
  - there may need to be trade offs if multiple instances use the same profile at the same time.