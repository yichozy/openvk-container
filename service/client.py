import openviking as ov

class OpenVK:
    _client = None
    
    @classmethod
    def get_client(cls):
        if cls._client is None:
            # Connect to remote services
            try:
                cls._client = ov.OpenViking(path="./data/workspace")
                cls._client.initialize()
            except Exception as e:
                print(f"Error connecting to OpenViking: {e}")

        return cls._client

    @classmethod
    def close_client(cls):
        if cls._client is not None:
            cls._client.close()
            cls._client = None

