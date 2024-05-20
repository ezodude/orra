import secrets

# noinspection SpellCheckingInspection
ENCODE_STR = "23456789abcdefghikmnopqrstuvwxyz"


def custom_base32_encode(data, encode_str):
    encoded = ''
    padding = (5 - len(data) % 5) % 5
    data += b'\x00' * padding
    for i in range(0, len(data), 5):
        h = [data[i+j] << (8*j) for j in range(5)]
        for k in range(8):
            encoded += encode_str[(h[0] >> (5*k)) & 31]
    return encoded[:len(encoded)-padding]


def gen_id():
    data = secrets.token_bytes(3)
    return custom_base32_encode(data, ENCODE_STR)

