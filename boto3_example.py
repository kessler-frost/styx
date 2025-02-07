import asyncio
from pprint import pprint as print
from aiobotocore.session import get_session

AWS_ACCESS_KEY_ID = "xxx"
AWS_SECRET_ACCESS_KEY = "xxx"


async def go():
    bucket = 'dataintake'
    filename = 'dummy.bin'
    folder = 'aiobotocore'
    key = f'{folder}/{filename}'

    session = get_session()
    async with session.create_client(
        's3',
        region_name='mordor',
        aws_secret_access_key=AWS_SECRET_ACCESS_KEY,
        aws_access_key_id=AWS_ACCESS_KEY_ID,
        endpoint_url="http://styx-prime:8333",
    ) as client:
        # upload object to amazon s3
        data = b'\x01' * 1024
        resp = await client.put_object(Bucket=bucket, Key=key, Body=data)
        print("PUT OBJECT RESPONSE:")
        print(resp)

        # getting s3 object properties of file we just uploaded
        resp = await client.get_object_acl(Bucket=bucket, Key=key)
        print("GET OBJECT ACL RESPONSE:")
        print(resp)

        resp = await client.get_object(Bucket=bucket, Key=key)
        async with resp['Body'] as stream:
            await stream.read()  # if you do not read the stream the connection cannot be re-used and will be dropped
            print("GET OBJECT RESPONSE:")
            print(resp)
        """
        This is to ensure the connection is returned to the pool as soon as possible.
        Otherwise the connection will be released after it is GC'd
        """

        # delete object from s3
        resp = await client.delete_object(Bucket=bucket, Key=key)
        print("DELETE OBJECT RESPONSE:")
        print(resp)


if __name__ == '__main__':
    asyncio.run(go())
