package org.chengbing.service.impl;

import com.baomidou.mybatisplus.core.conditions.query.QueryWrapper;
import com.baomidou.mybatisplus.core.conditions.update.UpdateWrapper;
import com.drew.imaging.ImageMetadataReader;
import com.drew.imaging.ImageProcessingException;
import com.drew.metadata.Directory;
import com.drew.metadata.Metadata;
import com.drew.metadata.Tag;
import org.apache.commons.io.FileUtils;
import org.chengbing.dao.*;
import org.chengbing.entity.*;
import org.chengbing.service.INamespacePhotoService;
import org.chengbing.service.IPhotoService;
import com.baomidou.mybatisplus.extension.service.impl.ServiceImpl;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Service;
import org.springframework.web.multipart.MultipartFile;

import javax.annotation.Resource;
import javax.imageio.ImageIO;
import javax.naming.Name;
import java.awt.*;
import java.awt.image.BufferedImage;
import java.io.*;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.UUID;

/**
 * <p>
 *  服务实现类
 * </p>
 *
 * @author HaroldCI
 * @since 2022-06-24
 */
@Service
public class PhotoServiceImpl extends ServiceImpl<PhotoMapper, Photo> implements IPhotoService {

    @Resource
    PhotoMapper mapper;

    @Resource
    UserMapper userMapper;

    @Resource
    TagPhotoMapper tagPhotoMapper;

    @Resource
    GalleryPhotoMapper galleryPhotoMapper;

    @Resource
    INamespacePhotoService namespacePhotoService;

    @Resource
    NamespaceMapper namespaceMapper;

    @Resource
    PhotoExtraMapper photoExtraMapper;

    @Value("${file.uploadFolder}")
    String uploadFolder;

    public void insertPhotoThumbnail(MultipartFile multipartFile, File saveFile, String format) throws IOException {
        String lowerFormat = format.toLowerCase();
        if (lowerFormat.equals("jpg") || lowerFormat.equals("jpeg") || lowerFormat.equals("gif") || lowerFormat.equals("png")
                || lowerFormat.equals("jfif") || lowerFormat.equals("ico") || lowerFormat.equals("bmp")
                || lowerFormat.equals("tif"))
        {
            InputStream inputStream = new ByteArrayInputStream(multipartFile.getBytes());
            BufferedImage bimg = ImageIO.read(inputStream);
            int width = bimg.getWidth();
            int height = bimg.getHeight();
            boolean compress = width > 400 || height > 250;
            int compressedWidth = compress ? 400 : width;
            int compressedHeight = compress ? 400 * height / width : height;

            BufferedImage img = new BufferedImage(compressedWidth, compressedHeight, BufferedImage.TYPE_INT_RGB);
            img.createGraphics().drawImage(bimg.getScaledInstance(compressedWidth, compressedHeight, Image.SCALE_SMOOTH),
                    0,0,null);

            ImageIO.write(img, format, saveFile);
            inputStream.close();
        }
    }

    @Override
    public Integer insertPhoto(Integer userId, MultipartFile file, Photo photo) {
        String uuid = userMapper.selectById(userId).getUuid();
        String fileName = file.getOriginalFilename();
        String suffix = fileName.substring(fileName.lastIndexOf(".")+1);
        Integer visibility = photo.getVisibility();
        if (visibility == null || visibility < 0 || visibility > 3)
            photo.setVisibility(0);
        String photoUUID = (UUID.randomUUID().toString());
        photo.setPhotoName(fileName.substring(0, fileName.lastIndexOf(".")));
        photo.setPhotoUuid(photoUUID);
        photo.setFormat(suffix);
        photo.setToken(UUID.randomUUID().toString());
        photo.setUploadDate(LocalDateTime.now());
        photo.setUserId(userId);

        QueryWrapper<Namespace> nsVerify = new QueryWrapper<>();
        nsVerify.eq("ns_id", photo.getNsId());
        nsVerify.eq("user_id", userId);
        Namespace namespace = namespaceMapper.selectOne(nsVerify);
        if (namespace == null || !namespace.getUserId().equals(userId))
            return -2;

        // String fileType = file.getContentType();
        String folderLoc = uploadFolder + System.getProperty("file.separator") + uuid;
        File file1 = new File(folderLoc);
        if (!file1.exists())
            file1.mkdir();
        String localPath = folderLoc + System.getProperty("file.separator") + photoUUID + "." + suffix;
        File photoFile = new File(localPath);
        try {
            insertPhotoThumbnail(file, new File(folderLoc + System.getProperty("file.separator") + photoUUID + "_thumbnail" + "." + suffix),
                    suffix);
            file.transferTo(photoFile);
            int res = 0;
            res += mapper.insert(photo);
            res += afterInsertionHelper(photoUUID, localPath);
            return res - 1;
        } catch (IOException e) {
            throw new RuntimeException(e);
        }
    }

    @Override
    public Integer insertPhotos(Integer userId, MultipartFile[] files, List<Photo> photos) {
        String uuid = userMapper.selectById(userId).getUuid();
        int total = photos.size();
        if (total != files.length)
            return -1;
        if (total == 0)
            return 0;
        int change = 0;
        QueryWrapper<Namespace> nsVerify = new QueryWrapper<>();
        nsVerify.eq("ns_id", photos.get(0).getNsId());
        nsVerify.eq("user_id", userId);
        Namespace namespace = namespaceMapper.selectOne(nsVerify);
        if (namespace == null || !namespace.getUserId().equals(userId))
            return -2;
        for (int i = 0; i < total; i ++)
        {
            MultipartFile file = files[i];
            Photo photo = photos.get(i);
            String fileName = file.getOriginalFilename();
            String suffix = fileName.substring(fileName.lastIndexOf(".")+1);
            Integer visibility = photo.getVisibility();
            if (visibility == null || visibility < 0 || visibility > 3)
                photo.setVisibility(0);
            String photoUUID = (UUID.randomUUID().toString());
            photo.setPhotoName(fileName.substring(0, fileName.lastIndexOf(".")));
            photo.setPhotoUuid(photoUUID);
            photo.setFormat(suffix);
            photo.setToken(UUID.randomUUID().toString());
            photo.setUploadDate(LocalDateTime.now());
            photo.setUserId(userId);

            // String fileType = file.getContentType();
            String folderLoc = uploadFolder + System.getProperty("file.separator") + uuid;
            File file1 = new File(folderLoc);
            if (!file1.exists())
                file1.mkdir();
            String localPath = folderLoc + System.getProperty("file.separator") + photoUUID + "." + suffix;
            File photoFile = new File(localPath);
            try {
                insertPhotoThumbnail(file, new File(folderLoc + System.getProperty("file.separator") + photoUUID + "_thumbnail" + "." + suffix),
                        suffix);
                file.transferTo(photoFile);
                change += mapper.insert(photo);
                afterInsertionHelper(photoUUID, localPath);
            } catch (IOException e) {
                throw new RuntimeException(e);
            }
        }
        return change;
    }

    @Override
    public String insertPhotoRU(Integer userId, MultipartFile file, Photo photo) {
        String uuid = userMapper.selectById(userId).getUuid();
        String fileName = file.getOriginalFilename();
        String suffix = fileName.substring(fileName.lastIndexOf(".")+1);
        Integer visibility = photo.getVisibility();
        if (visibility == null || visibility < 0 || visibility > 3)
            photo.setVisibility(0);
        String photoUUID = (UUID.randomUUID().toString());
        photo.setPhotoName(fileName.substring(0, fileName.lastIndexOf(".")));
        photo.setPhotoUuid(photoUUID);
        photo.setFormat(suffix);
        photo.setToken(UUID.randomUUID().toString());
        photo.setUploadDate(LocalDateTime.now());
        photo.setUserId(userId);

        QueryWrapper<Namespace> nsVerify = new QueryWrapper<>();
        nsVerify.eq("ns_id", photo.getNsId());
        nsVerify.eq("user_id", userId);
        Namespace namespace = namespaceMapper.selectOne(nsVerify);
        if (namespace == null || !namespace.getUserId().equals(userId))
            return null;

        // String fileType = file.getContentType();
        String folderLoc = uploadFolder + System.getProperty("file.separator") + uuid;
        File file1 = new File(folderLoc);
        if (!file1.exists())
            file1.mkdir();
        String localPath = folderLoc + System.getProperty("file.separator") + photoUUID + "." + suffix;
        File photoFile = new File(localPath);
        try {
            insertPhotoThumbnail(file, new File(folderLoc + System.getProperty("file.separator") + photoUUID + "_thumbnail" + "." + suffix),
                    suffix);
            file.transferTo(photoFile);
            afterInsertionHelper(photoUUID, localPath);
            int res = mapper.insert(photo);
            res += afterInsertionHelper(photoUUID, localPath);
            return res >= 1 ? photoUUID : null;
        } catch (IOException e) {
            throw new RuntimeException(e);
        }
    }

    @Override
    public List<String> insertPhotosRU(Integer userId, MultipartFile[] files, List<Photo> photos) {
        String uuid = userMapper.selectById(userId).getUuid();
        int total = photos.size();
        if (total != files.length)
            return null;
        if (total == 0)
            return null;
        QueryWrapper<Namespace> nsVerify = new QueryWrapper<>();
        nsVerify.eq("ns_id", photos.get(0).getNsId());
        nsVerify.eq("user_id", userId);
        Namespace namespace = namespaceMapper.selectOne(nsVerify);
        if (namespace == null || !namespace.getUserId().equals(userId))
            return null;
        List<String> uidList = new ArrayList<>();
        for (int i = 0; i < total; i ++)
        {
            MultipartFile file = files[i];
            Photo photo = photos.get(i);
            String fileName = file.getOriginalFilename();
            String suffix = fileName.substring(fileName.lastIndexOf(".")+1);
            Integer visibility = photo.getVisibility();
            if (visibility == null || visibility < 0 || visibility > 3)
                photo.setVisibility(0);
            String photoUUID = (UUID.randomUUID().toString());
            photo.setPhotoName(fileName.substring(0, fileName.lastIndexOf(".")));
            photo.setPhotoUuid(photoUUID);
            photo.setFormat(suffix);
            photo.setToken(UUID.randomUUID().toString());
            photo.setUploadDate(LocalDateTime.now());
            photo.setUserId(userId);

            // String fileType = file.getContentType();
            String folderLoc = uploadFolder + System.getProperty("file.separator") + uuid;
            File file1 = new File(folderLoc);
            if (!file1.exists())
                file1.mkdir();
            String localPath = folderLoc + System.getProperty("file.separator") + photoUUID + "." + suffix;
            File photoFile = new File(localPath);
            try {
                insertPhotoThumbnail(file, new File(folderLoc + System.getProperty("file.separator") + photoUUID + "_thumbnail" + "." + suffix),
                        suffix);
                file.transferTo(photoFile);
                int res = mapper.insert(photo);
                afterInsertionHelper(photoUUID, localPath);
                if (res >= 1)
                    uidList.add(photoUUID);
            } catch (IOException e) {
                throw new RuntimeException(e);
            }
        }
        return uidList;
    }

    @Override
    public Integer deletePhoto(Integer userId, Integer photoId) {
        QueryWrapper<Photo> wrapper = new QueryWrapper<>();
        wrapper.eq("photo_id", photoId);
        wrapper.eq("user_id", userId);
        Photo selected = mapper.selectOne(wrapper);
        if (selected!=null)
        {
            String uuid = userMapper.selectById(userId).getUuid();
            Photo photo = mapper.selectById(photoId);
            String photoUUID = photo.getPhotoUuid();
            String suffix = photo.getFormat();
            String localPath = uploadFolder + System.getProperty("file.separator") + uuid + System.getProperty("file.separator") + photoUUID + "." + suffix;

            File photoFile = new File(localPath);
            File photoFileThumbnail = new File(uploadFolder + System.getProperty("file.separator") + uuid +
                    System.getProperty("file.separator") + photoUUID + "_thumbnail" + "." + suffix);

            if (photoFile.exists())
            {
                boolean isDeleted = photoFile.delete();
                if (photoFileThumbnail.exists())
                {
                    boolean isDeletedThumbnail = photoFileThumbnail.delete();

                    if (isDeleted && isDeletedThumbnail)
                        return mapper.deleteById(photoId);
                }else {
                    if (isDeleted)
                        return mapper.deleteById(photoId);
                }
            }else {
                return mapper.deleteById(photoId);
            }

        }
        return -1;
    }

    @Override
    public Integer updatePhotoNameAndVisibility(Integer userId, Photo photo) {
        QueryWrapper<Photo> wrapper = new QueryWrapper<>();
        wrapper.eq("photo_id", photo.getPhotoId());
        wrapper.eq("user_id", userId);
        Photo selected = mapper.selectOne(wrapper);
        if (selected != null)
        {
            UpdateWrapper<Photo> updateWrapper = new UpdateWrapper<>();
            updateWrapper.eq("photo_id", photo.getPhotoId());
            if (photo.getPhotoName() != null)
                updateWrapper.set("photo_name", photo.getPhotoName());
            else
                updateWrapper.set("photo_name", selected.getPhotoName());
            if (photo.getVisibility() != null)
                updateWrapper.set("visibility", photo.getVisibility());
            else
                updateWrapper.set("visibility", selected.getVisibility());
            return mapper.update(null, updateWrapper);
        }
        return -1;
    }

    @Override
    public List<Photo> queryPhotoByNamespace(Integer userId, Integer nsId) {
        QueryWrapper<Photo> wrapper = new QueryWrapper<>();
        wrapper.eq("user_id", userId);
        wrapper.eq("ns_id", nsId);
        return mapper.selectList(wrapper);
    }

    @Override
    public Integer changeNamespace(Integer userId, Integer photoId, Integer nsId) {
        /*QueryWrapper<NamespacePhoto> wrapper = new QueryWrapper<>();
        wrapper.eq("photo_id", photoId);
        NamespacePhoto namespacePhoto = namespacePhotoMapper.selectOne(wrapper);
        namespacePhoto.setNsId(nsId);
        namespacePhotoMapper.updateById(namespacePhoto);*/

        QueryWrapper<Namespace> namespaceQueryWrapper = new QueryWrapper<>();
        namespaceQueryWrapper.eq("ns_id", nsId);
        namespaceQueryWrapper.eq("user_id", userId);
        Namespace namespace = namespaceMapper.selectOne(namespaceQueryWrapper);

        QueryWrapper<Photo> wrapper = new QueryWrapper<>();
        wrapper.eq("photo_id", photoId);
        wrapper.eq("user_id", userId);
        Photo selected = mapper.selectOne(wrapper);
        if (namespace != null && selected != null)
        {
            UpdateWrapper<Photo> updateWrapper = new UpdateWrapper<>();
            updateWrapper.eq("photo_id", photoId);
            updateWrapper.set("ns_id", nsId);
            return mapper.update(null, updateWrapper);
        }
        return -1;
    }


    @Override
    public List<Photo> queryPhotoByTags(Integer userId, List<Integer> tagIds) {
        return tagPhotoMapper.selectByTags(userId, tagIds, tagIds.size());
    }

    @Override
    public Integer addTags(Integer userId, Integer photoId, List<Integer> tagIds) {
        int res = 0;
        for (Integer tagId : tagIds)
        {
            QueryWrapper<TagPhoto> wrapper = new QueryWrapper<>();
            wrapper.eq("tag_id", tagId);
            wrapper.eq("user_id", userId);
            wrapper.eq("photo_id", photoId);
            if (tagPhotoMapper.selectOne(wrapper) == null)
                res += tagPhotoMapper.insert(new TagPhoto(userId, tagId, photoId));
        }
        return res;
    }

    @Override
    public Integer deleteTag(Integer userId, Integer photoId, Integer tagId) {
        QueryWrapper<TagPhoto> wrapper = new QueryWrapper<>();
        wrapper.eq("tag_id", tagId);
        wrapper.eq("user_id", userId);
        wrapper.eq("photo_id", photoId);
        return tagPhotoMapper.delete(wrapper);
    }

    @Override
    public List<Photo> queryPhotoByGallery(Integer userId, Integer gaId) {
        return galleryPhotoMapper.selectPhotoByGallery(gaId);
    }

    @Override
    public Integer addToGallery(Integer userId, Integer photoId, Integer gaId) {
        QueryWrapper<GalleryPhoto> wrapper = new QueryWrapper<>();
        wrapper.eq("ga_id", gaId);
        wrapper.eq("photo_id", photoId);
        if (galleryPhotoMapper.selectOne(wrapper) == null)
            return galleryPhotoMapper.insert(new GalleryPhoto(gaId, photoId));
        return -1;
    }

    @Override
    public Integer deleteFromGallery(Integer userId, Integer photoId, Integer gaId) {
        QueryWrapper<GalleryPhoto> wrapper = new QueryWrapper<>();
        wrapper.eq("ga_id", gaId);
        wrapper.eq("photo_id", photoId);
        return galleryPhotoMapper.delete(wrapper);
    }

    @Override
    public List<LinkedHashMap<String, Object>> selectTagByPhoto(Integer userId, Integer photoId) {
        QueryWrapper<Photo> wrapper = new QueryWrapper<>();
        wrapper.eq("photo_id", photoId);
        wrapper.eq("user_id", userId);
        Photo selected = mapper.selectOne(wrapper);
        if (selected != null)
        {
            return tagPhotoMapper.selectTagByPhoto(photoId);
        }
        return null;
    }

    @Override
    public List<LinkedHashMap<String, Object>> selectGalleryByPhoto(Integer userId, Integer photoId) {
        QueryWrapper<Photo> wrapper = new QueryWrapper<>();
        wrapper.eq("photo_id", photoId);
        wrapper.eq("user_id", userId);
        Photo selected = mapper.selectOne(wrapper);
        if (selected != null)
        {
            return galleryPhotoMapper.selectGalleryByPhoto(photoId);
        }
        return null;
    }

    @Override
    public List<Photo> searchPhoto(Integer userId, String option, String query) {
        List<Photo> retList = new ArrayList<>();
        switch (option) {
            case "photoName":
                QueryWrapper<Photo> wrapper = new QueryWrapper<>();
                wrapper.like("photo_name", query);
                wrapper.eq("user_id", userId);
                retList = mapper.selectList(wrapper);
                break;
            case "tag":
                retList = tagPhotoMapper.queryPhotoByTagName(userId, query);
                break;
            case "gallery":
                retList = galleryPhotoMapper.queryPhotoByGaName(userId, query);
                break;
            case "namespace":
                retList = namespacePhotoService.queryPhotoByNsName(userId, query);
                break;
        }

        return retList;
    }

    private LinkedHashMap<String, String> getPhotoExifHelper(String fileAddr)
    {
        File file = new File(fileAddr);
        if (file.exists())
        {
            try {
                LinkedHashMap<String, String> res = new LinkedHashMap<>();
                Metadata metadata = ImageMetadataReader.readMetadata(file);
                for(Directory directory : metadata.getDirectories())
                {
                    for(Tag imageExifTag : directory.getTags())
                    {
                        String exifTagName = imageExifTag.getTagName();
                        String desc = imageExifTag.getDescription();
                        boolean put = false;

                        boolean gpsLongitudeNeg = false;
                        boolean gpsLatitudeNeg = false;

                        switch (exifTagName) {
                            case "GPS Altitude":
                            case "GPS Altitude Ref":
                            case "GPS Version ID":
                            case "Artist":
                            case "Lens Make":
                            case "Lens Model":
                            case "Color Space":
                            case "Focal Length":
                            case "Shutter Speed Value":
                            case "ISO Speed Ratings":
                            case "Aperture Value":
                            case "Image Height":
                            case "Y Resolution":
                            case "X Resolution":
                            case "Model":
                            case "Make":
                            case "Data Precision":
                            case "Image Width":
                                put = true;
                                break;
                            case "Date/Time":
                                put = true;
                                exifTagName = "Time";
                                break;
                            case "File Size":
                                put = true;
                                int numBytes = Integer.parseInt(desc.split(" ")[0]);
                                double numMB = numBytes / 1024.00 / 1024.00;
                                desc = String.format("%.2f", numMB);
                                desc = desc + " MB";
                                break;
                            case "GPS Longitude Ref":
                                if (desc.equals("W"))
                                    gpsLongitudeNeg = true;
                                put = true;
                                break;
                            case "GPS Latitude Ref":
                                if (desc.equals("S"))
                                    gpsLatitudeNeg = true;
                                put = true;
                                break;
                            case "GPS Longitude":
                            case "GPS Latitude":
                                put = true;
                                double d = Double.parseDouble(desc.split("°")[0]);
                                double m = Double.parseDouble(desc.split(" ")[1].replace("'", ""));
                                double s = Double.parseDouble(desc.split(" ")[2].replace("\"", ""));
                                double dd = Math.signum(d) * (Math.abs(d) + (m / 60.0) + (s / 3600.0));
                                desc = String.format("%.4f", dd);
                        }
                        if (put)
                            res.put(exifTagName.replace(' ', '_'), desc);
                    }
                }
                return res;
            } catch (ImageProcessingException | IOException e) {
                throw new RuntimeException(e);
            }
        }
        return null;
    }

    private int afterInsertionHelper(String photoUUID, String photoAddr)
    {
        LinkedHashMap<String, String> exifInfo = getPhotoExifHelper(photoAddr);
        PhotoExtra photoExtra = new PhotoExtra();
        if (exifInfo != null)
        {
            if (exifInfo.get("GPS_Longitude") != null)
            {
                photoExtra.setExtraPhotoGpsLon(Double.valueOf(exifInfo.get("GPS_Longitude")));
            }
            if (exifInfo.get("GPS_Latitude") != null)
            {
                photoExtra.setExtraPhotoGpsLat(Double.valueOf(exifInfo.get("GPS_Latitude")));
            }
            if (exifInfo.get("Make") != null && exifInfo.get("Model") != null)
            {
                photoExtra.setExtraPhotoCamera(exifInfo.get("Make") + " " + exifInfo.get("Model"));
            }
            if (exifInfo.get("Lens_Make") != null && exifInfo.get("Lens_Model") != null)
            {
                photoExtra.setExtraPhotoLens(exifInfo.get("Lens_Make") + " " + exifInfo.get("Lens_Model"));
            }else if (exifInfo.get("Lens_Make") != null)
            {
                photoExtra.setExtraPhotoLens(exifInfo.get("Lens_Make"));
            }else if (exifInfo.get("Lens_Model") != null)
            {
                photoExtra.setExtraPhotoLens(exifInfo.get("Lens_Model"));
            }
            if (exifInfo.get("Focal_Length") != null)
            {
                double res = Double.parseDouble(exifInfo.get("Focal_Length").split("mm")[0].trim());
                photoExtra.setExtraPhotoFocal((int) res);
            }
            if (exifInfo.get("Aperture_Value") != null)
            {
                photoExtra.setExtraPhotoAperture(Double.valueOf(exifInfo.get("Aperture_Value").split("f\\/")[1].trim()));
            }
            if (exifInfo.get("Shutter_Speed_Value") != null)
            {
                try {
                    photoExtra.setExtraPhotoShutterSec(Integer.valueOf(exifInfo.get("Shutter_Speed_Value").split("sec")[0].trim().split("\\/")[1]));
                }catch (Exception e)
                {
                    e.printStackTrace();
                }
            }
            if (exifInfo.get("ISO_Speed_Ratings") != null)
            {
                photoExtra.setExtraPhotoIso(Integer.valueOf(exifInfo.get("ISO_Speed_Ratings")));
            }
            if (exifInfo.get("Time") != null)
            {
                String exifTime = exifInfo.get("Time");
                String date = exifTime.split(" ")[0];
                date = date.replace(":", "-");
                String sec = exifTime.split(" ")[1];
                photoExtra.setExtraPhotoTakenDate(LocalDateTime.parse(date + "T" + sec));
            }
        }

        QueryWrapper<Photo> wrapper = new QueryWrapper<>();
        wrapper.eq("photo_uuid", photoUUID);
        Photo selected = mapper.selectOne(wrapper);

        Integer photoId = selected.getPhotoId();
        photoExtra.setFPhotoId(photoId);

        return photoExtraMapper.insert(photoExtra);
    }

    @Override
    public LinkedHashMap<String, String> getPhotoExif(Integer userId, Integer photoId, String userUUID)
    {
        QueryWrapper<Photo> wrapper = new QueryWrapper<>();
        wrapper.eq("photo_id", photoId);
        wrapper.eq("user_id", userId);
        Photo selected = mapper.selectOne(wrapper);
        if (selected!=null)
        {
            // Get MetaInfo of this image
            String addr = uploadFolder + System.getProperty("file.separator") + userUUID + System.getProperty("file.separator") + selected.getPhotoUuid()
                    + "." + selected.getFormat();
            return getPhotoExifHelper(addr);
        }
        return null;
    }
}
