import openviking as ov

class OpenVK:
    
    @staticmethod
    def get_client(cls):

        # Connect to remote services
        client = ov.OpenViking(path="/data/workspace")
        client.initialize()

        return client

