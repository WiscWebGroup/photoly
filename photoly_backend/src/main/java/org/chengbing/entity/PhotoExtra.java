package org.chengbing.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import java.time.LocalDateTime;
import java.io.Serializable;
import io.swagger.annotations.ApiModel;
import io.swagger.annotations.ApiModelProperty;
import lombok.Data;
import lombok.EqualsAndHashCode;

/**
 * <p>
 * 
 * </p>
 *
 * @author HaroldCI
 * @since 2024-03-04
 */
@Data
@EqualsAndHashCode(callSuper = false)
@ApiModel(value="PhotoExtra对象", description="")
public class PhotoExtra implements Serializable {

    private static final long serialVersionUID = 1L;

    @TableId(value = "extra_photo_id", type = IdType.AUTO)
    private Integer extraPhotoId;

    private Integer fPhotoId;

    private Double extraPhotoGpsLat;

    private Double extraPhotoGpsLon;

    private String extraPhotoCamera;

    private String extraPhotoLens;

    private Double extraPhotoAperture;

    private Integer extraPhotoFocal;

    private Integer extraPhotoIso;

    @ApiModelProperty(value = "this is the int for 1/<num>")
    private Integer extraPhotoShutterSec;

    private LocalDateTime extraPhotoTakenDate;

    @ApiModelProperty(value = "a simple note for the photo")
    private String extraPhotoNote;


}
