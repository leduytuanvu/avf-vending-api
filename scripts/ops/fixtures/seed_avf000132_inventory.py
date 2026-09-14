#!/usr/bin/env python3
"""Write avf000132-mt01-layout1.inventory.json fixture from known AVF000132 layout."""

import json
from pathlib import Path

COMMON = {
    "machine_code": "AVF000132",
    "cabinet_code": "MT01",
    "product_layout_id": "1",
    "android_id": "23f886b4d8d09758",
    "spring_type": "lo xo don",
    "is_enable": True,
    "is_lock": False,
    "mess_drop": "",
}

ROWS = [
    (1, "6551005630793120260914102423426", 7, 5, 1, True, 1, 10000, "1702", "Lays Wavy vị bò bít tết 32gr"),
    (2, "5842641483973320260424111845245", 6, 6, 0, False, 0, 10000, "", ""),
    (3, "9339240110455920260911152157602", 7, 7, 1, True, 3, 10000, "1702", "Lays Wavy vị bò bít tết 32gr"),
    (4, "9454415626394320260424111845245", 6, 6, 0, False, 0, 10000, "", ""),
    (5, "1990558666065220260914093917258", 12, 9, 1, False, 0, 5000, "6906", "Bánh Solo kem bơ sữa gói 18g"),
    (6, "9416897764800720260910103600798", 12, 12, 1, False, 0, 5000, "6907", "Bánh Solo kem bơ lá dứa gói 18g"),
    (7, "3292304429492720260907151014215", 7, 7, 1, False, 0, 15000, "6900", "Bánh Sừng Bò Bauli Moonfils Nhân Sôcôla"),
    (8, "8600988736865520260911152157603", 7, 7, 1, False, 0, 15000, "6900", "Bánh Sừng Bò Bauli Moonfils Nhân Sôcôla"),
    (9, "9978310364878620260911152157603", 6, 6, 1, True, 9, 15000, "3701", "Mì Ly Modern - Lẩu thái tôm"),
    (10, "7824995032351920260424111845247", 6, 6, 0, False, 0, 10000, "", "Not have product"),
    (11, "7159234507148720260911152157604", 6, 6, 1, False, 0, 15000, "2115", "Juicy Milk Dâu 320ml"),
    (12, "1170938924027220260908152535382", 6, 6, 1, False, 0, 15000, "2115", "Juicy Milk Dâu 320ml"),
    (13, "3193833621510720260909114348748", 6, 5, 1, False, 0, 15000, "2106", "Twister Cam 350ml/320ml"),
    (14, "5361767288644220260911064845866", 6, 5, 1, False, 0, 15000, "2106", "Twister Cam 350ml/320ml"),
    (15, "3837503196752720260914091913926", 6, 4, 1, False, 0, 10000, "2107", "Trà ô long plus 350ml/320ml"),
    (16, "9299396642877020260914052900964", 6, 5, 1, False, 0, 10000, "2107", "Trà ô long plus 350ml/320ml"),
    (17, "9772470122003520260912062343503", 6, 5, 1, False, 0, 10000, "4102", "Trà Boncha vị Tắc PET 450ml"),
    (18, "2065422250266220260910133044965", 6, 6, 1, False, 0, 10000, "4102", "Trà Boncha vị Tắc PET 450ml"),
    (19, "7348647728238620260911152157605", 6, 6, 1, False, 0, 10000, "4103", "Trà Boncha vị Việt Quất PET 450ml"),
    (20, "7491570466861720260913144853269", 6, 4, 1, False, 0, 10000, "4103", "Trà Boncha vị Việt Quất PET 450ml"),
    (21, "6740120914151120260910103600801", 6, 6, 1, False, 0, 10000, "7202", "Manuka Trà Mật Ong Vị Chanh"),
    (22, "2542794235960120260910103600801", 6, 6, 1, False, 0, 10000, "7203", "Manuka Trà Mật Ong Vị Đào"),
    (23, "5030059212126620260914065404186", 6, 4, 1, False, 0, 15000, "2133", "Trà Xanh Ô Long Tea Plus 450ml"),
    (24, "6637175238903720260911092355357", 6, 5, 1, False, 0, 15000, "2133", "Trà Xanh Ô Long Tea Plus 450ml"),
    (25, "8764394377797920260914093917258", 6, 3, 1, False, 0, 10000, "2222", "Latte Socola"),
    (26, "5807386869979920260912173347869", 6, 5, 1, False, 0, 10000, "2222", "Latte Socola"),
    (27, "1563520199120920260914091413901", 6, 3, 1, False, 0, 10000, "2212", "Latte Trà 440ml"),
    (28, "8302595198910120260911164703978", 6, 5, 1, False, 0, 10000, "2212", "Latte Trà 440ml"),
    (29, "7282012476987220260910133044966", 6, 6, 1, False, 0, 15000, "5003", "Nước ép đào thạch dừa TENRO"),
    (30, "6624120210501220260910133044967", 6, 6, 1, False, 0, 15000, "5003", "Nước ép đào thạch dừa TENRO"),
    (31, "9702616855499520260914061401098", 6, 0, 1, False, 0, 10000, "3205", "Lavie dịu nhẹ"),
    (32, "8394278693918720260914093917259", 6, 1, 1, False, 0, 10000, "3205", "Lavie dịu nhẹ"),
    (33, "4075725930846820260914080413671", 6, 5, 1, False, 0, 10000, "3205", "Lavie dịu nhẹ"),
    (34, "9034300813397520260910103600803", 6, 6, 1, False, 0, 10000, "3205", "Lavie dịu nhẹ"),
    (35, "4324766711573220260914102423426", 6, 3, 1, False, 0, 10000, "3205", "Lavie dịu nhẹ"),
    (36, "7704551628278120260914070910496", 6, 4, 1, False, 0, 10000, "3205", "Lavie dịu nhẹ"),
    (37, "7001083152996520260914100920386", 6, 2, 1, False, 0, 10000, "3205", "Lavie dịu nhẹ"),
    (38, "1878508483782720260914103423465", 6, 4, 1, False, 0, 10000, "3205", "Lavie dịu nhẹ"),
    (39, "7258446784230720260914090913875", 6, 3, 1, False, 0, 10000, "3205", "Lavie dịu nhẹ"),
    (40, "2585728652200720260914090913875", 6, 2, 1, False, 0, 10000, "3205", "Lavie dịu nhẹ"),
    (41, "3538212398027820260911063345814", 7, 6, 1, False, 0, 10000, "5301", "Sữa tiệt trùng Lothamilk Dâu 180ml"),
    (42, "1548277054235720260914102423426", 7, 5, 1, False, 0, 10000, "5302", "Sữa tiệt trùng Lothamilk Cam 180ml"),
    (43, "8097161842556720260914102423427", 7, 4, 1, False, 0, 10000, "6502", "Sữa yến mạch vị sô-cô-la Oatside 180ml"),
    (44, "1723849164855820260911152157609", 7, 7, 1, False, 0, 10000, "6502", "Sữa yến mạch vị sô-cô-la Oatside 180ml"),
    (45, "4805950662710720260911152157610", 7, 7, 1, False, 0, 10000, "7204", "Sữa Bắp Non LOF Hộp 180ml"),
    (46, "4663216109648820260913143353221", 6, 5, 1, False, 0, 10000, "7801", "Trà Bí Đao A Nuta Chai 360ml"),
    (47, "3402366494147020260913200856935", 6, 3, 1, False, 0, 10000, "1201", "C2 Green Tea Lemon 360ml/355ml"),
    (48, "9446277898336220260914065404187", 7, 4, 1, False, 0, 20000, "5602", "Cà Phê Sữa Highlands Coffee 235ml"),
    (49, "9788883920479220260914073413555", 6, 4, 1, False, 0, 15000, "2507", "Redbull Thailand"),
    (50, "7381240324806720260914073413555", 6, 4, 1, False, 0, 15000, "2507", "Redbull Thailand"),
    (51, "1439402654907820260914065404188", 6, 5, 1, False, 0, 15000, "6302", "Nước Yến Nha Đam Tingco"),
    (52, "6916761978570720260911154203797", 6, 5, 1, False, 0, 15000, "6302", "Nước Yến Nha Đam Tingco"),
    (53, "1818825177108420260911131902044", 6, 0, 1, False, 0, 10000, "5501", "Davis Dừa Nha Đam"),
    (54, "9154755416733820260914073413555", 6, 0, 1, False, 0, 10000, "5501", "Davis Dừa Nha Đam"),
    (55, "3158875183931520260911152157610", 6, 5, 1, False, 0, 15000, "2105", "7UP Revive 500ml"),
    (56, "5086738836490420260910103600807", 6, 6, 1, False, 0, 15000, "2105", "7UP Revive 500ml"),
    (57, "5942746927818420260914070910497", 6, 5, 1, False, 0, 10000, "7802", "Nước Chanh Muối Restore Chai 495ml"),
    (58, "9480763406569220260914082413742", 6, 5, 1, False, 0, 10000, "7802", "Nước Chanh Muối Restore Chai 495ml"),
    (59, "3860557801318820260913190856806", 6, 5, 1, False, 0, 15000, "2103", "Sting dâu chai 330ml"),
    (60, "3006411908073220260914073413556", 6, 3, 1, False, 0, 10000, "2108", "Pepsi chai 390ml"),
]


def build_docs() -> list[dict]:
    docs = []
    for slot, doc_id, capacity, inventory, is_active, is_combine, slot_combine, price, product_code, product_name in ROWS:
        docs.append(
            {
                **COMMON,
                "slot": slot,
                "id": doc_id,
                "capacity": capacity,
                "inventory": inventory,
                "remaining": inventory,
                "is_active": is_active,
                "is_combine": is_combine,
                "slot_combine": slot_combine,
                "price": price,
                "product_code": product_code,
                "product_name": product_name,
                "status": str(is_active),
            }
        )
    return docs


def main() -> None:
    out = Path(__file__).resolve().parent / "avf000132-mt01-layout1.inventory.json"  # regenerate fixture
    out.write_text(json.dumps(build_docs(), ensure_ascii=False, indent=2), encoding="utf-8")
    print(f"Wrote {out} ({len(ROWS)} docs)")


if __name__ == "__main__":
    main()
