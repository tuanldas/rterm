/// Tên sản phẩm và các giá trị gắn với tên đó, khai báo một chỗ cho app.
///
/// Sản phẩm còn đổi tên trước khi phát hành, nên mọi nơi trong app lấy giá trị
/// từ đây thay vì viết lại chuỗi. Giá trị phải khớp `product.json` ở gốc repo;
/// `test/core/product_test.dart` đối chiếu và chỉ ra chỗ sót (kể cả
/// `applicationId` và nhãn app của Android, vốn bắt buộc khai báo ở Gradle và
/// manifest).
///
/// Định danh trên dây (tên method, capability, biến môi trường mang mã phiên)
/// KHÔNG lấy từ đây: chúng thuộc hợp đồng giao thức và phải giữ nguyên qua lần
/// đổi tên vì agent và server đã phát hành vẫn chạy.
library;

/// Tên sản phẩm hiển thị cho người dùng. Không dịch nên không nằm trong ARB.
const productName = 'rterm';

/// Định danh gói của app Android (`applicationId`).
const appId = 'com.tuanldas.rterm';

/// Scheme của deep link (lời mời, thiết lập server).
const urlScheme = 'rterm';
