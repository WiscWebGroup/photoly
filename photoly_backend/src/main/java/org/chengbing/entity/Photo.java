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
 * @since 2022-06-24
 */
@Data
@EqualsAndHashCode(callSuper = false)
@ApiModel(value="Photo对象", description="")
public class Photo implements Serializable {

    private static final long serialVersionUID = 1L;

    @TableId(value = "photo_id", type = IdType.AUTO)
    private Integer photoId;

    private String photoName;

    private LocalDateTime uploadDate;

    private String format;

    private Integer nsId;

    private Integer visibility;

    private String token;


}
